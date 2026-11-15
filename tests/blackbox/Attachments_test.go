//go:build blackbox

package blackbox

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"testing"
)

type attachmentStart struct {
	FileName  string `json:"fileName"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Sha256    string `json:"sha256"`
}

type attachmentUpload struct {
	AttachmentId string `json:"attachmentId"`
	UploadUrl    string `json:"uploadUrl"`
	ExpiresAt    string `json:"expiresAt"`
}

type attachmentView struct {
	AttachmentId string `json:"attachmentId"`
	FileName     string `json:"fileName"`
	MediaType    string `json:"mediaType"`
	Size         int64  `json:"size"`
	Sha256       string `json:"sha256"`
	State        string `json:"state"`
}

type attachmentDownload struct {
	Url       string `json:"url"`
	ExpiresAt string `json:"expiresAt"`
}

type uploadedAttachment struct {
	Id      string
	Content []byte
	Url     string
}

const attachmentsPath = "/api/v1/attachments"

const attachmentFileSize = 300 * 1024

func randomContent(t *testing.T, size int) []byte {
	t.Helper()
	content := make([]byte, size)
	if _, err := rand.Read(content); err != nil {
		t.Fatal(err)
	}
	return content
}

func attachmentStartFor(content []byte) attachmentStart {
	sum := sha256.Sum256(content)
	return attachmentStart{FileName: "op7-blackbox.bin", MediaType: "application/octet-stream", Size: int64(len(content)), Sha256: hex.EncodeToString(sum[:])}
}

func uploadAttachment(t *testing.T, stack Stack, token string, content []byte) uploadedAttachment {
	t.Helper()
	var upload attachmentUpload
	stack.Call(t, http.MethodPost, attachmentsPath, token, attachmentStartFor(content)).Expect(t, http.StatusCreated).Decode(t, &upload)
	if status, body := sendBytes(t, http.MethodPut, upload.UploadUrl, content); status != http.StatusOK {
		t.Fatalf("upload through /blobs answered %d: %s", status, body)
	}
	return uploadedAttachment{Id: upload.AttachmentId, Content: content, Url: upload.UploadUrl}
}

func completeAttachment(t *testing.T, stack Stack, token string, upload uploadedAttachment) attachmentView {
	t.Helper()
	var attachment attachmentView
	stack.Call(t, http.MethodPost, attachmentsPath+"/"+upload.Id+"/complete", token, nil).Expect(t, http.StatusOK).Decode(t, &attachment)
	return attachment
}

func downloadLink(t *testing.T, stack Stack, token string, attachmentId string) string {
	t.Helper()
	var download attachmentDownload
	stack.Call(t, http.MethodGet, attachmentsPath+"/"+attachmentId, token, nil).Expect(t, http.StatusOK).Decode(t, &download)
	return download.Url
}

func changedLink(t *testing.T, link string, name string, value string) string {
	t.Helper()
	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set(name, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func expectRefused(t *testing.T, method string, link string, body []byte) {
	t.Helper()
	status, answer := sendBytes(t, method, link, body)
	if status != http.StatusForbidden {
		t.Fatalf("%s %s: want 403, got %d: %s", method, link, status, answer)
	}
}

func readyAttachment(t *testing.T, stack Stack, token string) uploadedAttachment {
	t.Helper()
	upload := uploadAttachment(t, stack, token, randomContent(t, attachmentFileSize))
	attachment := completeAttachment(t, stack, token, upload)
	if attachment.State != "ready" || attachment.Size != attachmentFileSize {
		t.Fatalf("want a ready attachment of %d bytes, got %+v", attachmentFileSize, attachment)
	}
	return upload
}

func TestOp7AnAttachmentUploadsAndDownloadsThroughTheRouter(t *testing.T) {
	stack := NewStack(t)
	_, token := backupPerson(t, stack)
	upload := readyAttachment(t, stack, token)
	status, body := sendBytes(t, http.MethodGet, downloadLink(t, stack, token, upload.Id), nil)
	if status != http.StatusOK || len(body) != attachmentFileSize || !bytes.Equal(body, upload.Content) {
		t.Fatalf("download answered %d with %d bytes; want 200 with the %d bytes sent", status, len(body), attachmentFileSize)
	}
}

func TestOp7AnotherDomainsMemberGetsNotFound(t *testing.T) {
	stack := NewStack(t)
	_, token := backupPerson(t, stack)
	upload := readyAttachment(t, stack, token)
	eve := fixture(t, stack, "EVE")
	stack.Call(t, http.MethodGet, attachmentsPath+"/"+upload.Id, eve.AccessToken, nil).ExpectError(t, http.StatusNotFound, "not_found")
	stack.Call(t, http.MethodPost, attachmentsPath+"/"+upload.Id+"/complete", eve.AccessToken, nil).ExpectError(t, http.StatusNotFound, "not_found")
}

func TestOp7AChangedLinkOrAPastExpiryIsRefused(t *testing.T) {
	stack := NewStack(t)
	_, token := backupPerson(t, stack)
	upload := readyAttachment(t, stack, token)
	link := downloadLink(t, stack, token, upload.Id)
	expectRefused(t, http.MethodGet, changedLink(t, link, "signature", "0000"), nil)
	expectRefused(t, http.MethodGet, changedLink(t, link, "file", "other.bin"), nil)
	expectRefused(t, http.MethodGet, changedLink(t, link, "expires", "1"), nil)
	expectRefused(t, http.MethodPut, changedLink(t, upload.Url, "expires", "1"), upload.Content)
}
