//go:build blackbox

package blackbox

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"testing"
	"time"
)

type backupKeySlot struct {
	Kind       string `json:"kind"`
	Salt       string `json:"salt"`
	Rounds     int64  `json:"rounds"`
	WrappedKey string `json:"wrappedKey"`
}

type backupStart struct {
	ExpectedVersion int64           `json:"expectedVersion"`
	Size            int64           `json:"size"`
	Sha256          string          `json:"sha256"`
	KeySlots        []backupKeySlot `json:"keySlots"`
	DeviceId        string          `json:"deviceId"`
}

type backupUpload struct {
	Version   int64  `json:"version"`
	UploadUrl string `json:"uploadUrl"`
	ExpiresAt string `json:"expiresAt"`
}

type backupView struct {
	Version           int64           `json:"version"`
	Size              int64           `json:"size"`
	Sha256            string          `json:"sha256"`
	KeySlots          []backupKeySlot `json:"keySlots"`
	UpdatedAt         string          `json:"updatedAt"`
	UpdatedByDeviceId string          `json:"updatedByDeviceId"`
}

type backupDownload struct {
	Url       string `json:"url"`
	ExpiresAt string `json:"expiresAt"`
}

const backupsPath = "/api/v1/attachments/backups/mine"

func backupStartFor(version int64, ciphertext []byte, deviceId string) backupStart {
	sum := sha256.Sum256(ciphertext)
	slot := backupKeySlot{
		Kind: "recovery", Rounds: 600000,
		Salt:       base64.RawURLEncoding.EncodeToString([]byte("blackbox-salt-16")),
		WrappedKey: base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{7}, 60)),
	}
	return backupStart{ExpectedVersion: version, Size: int64(len(ciphertext)), Sha256: hex.EncodeToString(sum[:]), KeySlots: []backupKeySlot{slot}, DeviceId: deviceId}
}

func sendBytes(t *testing.T, method string, url string, body []byte) (int, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/octet-stream")
	}
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	return response.StatusCode, raw
}

func backedUp(t *testing.T, stack Stack, token string, version int64, ciphertext []byte) backupView {
	t.Helper()
	var upload backupUpload
	stack.Call(t, http.MethodPut, backupsPath, token, backupStartFor(version, ciphertext, "blackbox-phone")).Expect(t, http.StatusCreated).Decode(t, &upload)
	if status, body := sendBytes(t, http.MethodPut, upload.UploadUrl, ciphertext); status >= 300 {
		t.Fatalf("upload answered %d: %s", status, body)
	}
	var backup backupView
	stack.Call(t, http.MethodPost, backupsPath+"/complete", token, map[string]int64{"version": upload.Version}).Expect(t, http.StatusOK).Decode(t, &backup)
	return backup
}

func backupPerson(t *testing.T, stack Stack) (newPerson, string) {
	t.Helper()
	ana := fixture(t, stack, "ANA")
	person := createPerson(t, stack, ana.AccessToken)
	return person, signIn(t, stack, person.Username, person.Password).AccessToken
}

func TestG4APersonBacksUpCompletesAndDownloads(t *testing.T) {
	stack := NewStack(t)
	_, token := backupPerson(t, stack)
	stack.Call(t, http.MethodGet, backupsPath, token, nil).ExpectError(t, http.StatusNotFound, "not_found")
	ciphertext := bytes.Repeat([]byte("sealed backup "), 100)
	backup := backedUp(t, stack, token, 0, ciphertext)
	if backup.Version != 1 || backup.Size != int64(len(ciphertext)) || backup.UpdatedByDeviceId != "blackbox-phone" || len(backup.KeySlots) != 1 {
		t.Fatalf("unexpected backup %+v", backup)
	}
	var mine backupView
	stack.Call(t, http.MethodGet, backupsPath, token, nil).Expect(t, http.StatusOK).Decode(t, &mine)
	if mine.Version != 1 || mine.Sha256 != backup.Sha256 {
		t.Fatalf("GET mine %+v, want %+v", mine, backup)
	}
	var download backupDownload
	stack.Call(t, http.MethodGet, backupsPath+"/download", token, nil).Expect(t, http.StatusOK).Decode(t, &download)
	status, body := sendBytes(t, http.MethodGet, download.Url, nil)
	if status != http.StatusOK || !bytes.Equal(body, ciphertext) {
		t.Fatalf("download answered %d with %d bytes", status, len(body))
	}
	second := backedUp(t, stack, token, 1, []byte("the next sealed backup"))
	if second.Version != 2 {
		t.Fatalf("want version 2, got %+v", second)
	}
	stack.Call(t, http.MethodDelete, backupsPath, token, nil).Expect(t, http.StatusNoContent)
	stack.Call(t, http.MethodGet, backupsPath, token, nil).ExpectError(t, http.StatusNotFound, "not_found")
	stack.Call(t, http.MethodDelete, backupsPath, token, nil).Expect(t, http.StatusNoContent)
}

func TestG4AStaleVersionIsAConflict(t *testing.T) {
	stack := NewStack(t)
	_, token := backupPerson(t, stack)
	backedUp(t, stack, token, 0, []byte("from the phone"))
	reply := stack.Call(t, http.MethodPut, backupsPath, token, backupStartFor(0, []byte("from the laptop"), "blackbox-laptop"))
	reply.ExpectError(t, http.StatusConflict, "conflict")
	if field := reply.Error(t).Field; field != "expectedVersion" {
		t.Fatalf("want field expectedVersion, got %q", field)
	}
	var mine backupView
	stack.Call(t, http.MethodGet, backupsPath, token, nil).Expect(t, http.StatusOK).Decode(t, &mine)
	if mine.Version != 1 || mine.UpdatedByDeviceId != "blackbox-phone" {
		t.Fatalf("the conflict changed the backup: %+v", mine)
	}
}

func TestG4AnotherPersonGetsNotFound(t *testing.T) {
	stack := NewStack(t)
	_, owner := backupPerson(t, stack)
	backedUp(t, stack, owner, 0, []byte("private history"))
	_, other := backupPerson(t, stack)
	stack.Call(t, http.MethodGet, backupsPath, other, nil).ExpectError(t, http.StatusNotFound, "not_found")
	stack.Call(t, http.MethodGet, backupsPath+"/download", other, nil).ExpectError(t, http.StatusNotFound, "not_found")
	stack.Call(t, http.MethodGet, backupsPath, owner, nil).Expect(t, http.StatusOK)
}

func TestG4RemovingTheMemberRemovesTheBackup(t *testing.T) {
	stack := NewStack(t)
	ana := fixture(t, stack, "ANA")
	person, token := backupPerson(t, stack)
	backedUp(t, stack, token, 0, []byte("history of a member who leaves"))
	token = signIn(t, stack, person.Username, person.Password).AccessToken
	removeMemberReply(t, stack, ana.AccessToken, person.Id).Expect(t, http.StatusNoContent)
	stack.Eventually(t, 45*time.Second, "the removed member's backup is gone", func() bool {
		status := stack.Call(t, http.MethodGet, backupsPath, token, nil).Status
		if status == http.StatusUnauthorized {
			t.Fatal("the token ended before the backup was removed")
		}
		return status == http.StatusNotFound
	})
}
