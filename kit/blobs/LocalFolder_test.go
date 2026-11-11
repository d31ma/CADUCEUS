package blobs

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testFolder(t *testing.T) (LocalFolder, string) {
	root := t.TempDir()
	return NewLocalFolder(root, "http://127.0.0.1:8105/api/v1/attachments/blobs", NewLocalSigner([]byte("test secret"))), root
}

func testNow() time.Time {
	return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
}

func grantFromUrl(t *testing.T, folder LocalFolder, method string, signed string, now time.Time) (Grant, error) {
	parsed, err := url.Parse(signed)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(strings.TrimPrefix(parsed.Path, "/api/v1/attachments/blobs/"), "/")
	return folder.Grant(method, Key{DomainId: parts[0], Name: parts[1]}, parsed.Query(), now)
}

func TestWriteThenReadReturnsTheBytes(t *testing.T) {
	folder, root := testFolder(t)
	key := Key{DomainId: "example.com", Name: "a1"}
	if err := folder.Write(key, strings.NewReader("hello"), 5); err != nil {
		t.Fatal(err)
	}
	reader, err := folder.Read(key)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	body, _ := io.ReadAll(reader)
	if string(body) != "hello" {
		t.Fatalf("got %q", body)
	}
	if _, err := os.Stat(filepath.Join(root, "example.com", "a1")); err != nil {
		t.Fatalf("blob is not under its domain folder: %v", err)
	}
}

func TestWriteRefusesTheWrongSizeAndKeepsNothing(t *testing.T) {
	folder, root := testFolder(t)
	key := Key{DomainId: "example.com", Name: "a1"}
	for _, body := range []string{"hell", "hello!"} {
		var mismatch SizeMismatch
		if err := folder.Write(key, strings.NewReader(body), 5); !errors.As(err, &mismatch) {
			t.Fatalf("%q: got %v", body, err)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(root, "example.com"))
	if len(entries) != 0 {
		t.Fatalf("left %d files", len(entries))
	}
}

func TestWriteRefusesABadKey(t *testing.T) {
	folder, _ := testFolder(t)
	var bad BadKey
	if err := folder.Write(Key{DomainId: "example.com", Name: "../escape"}, strings.NewReader("x"), 1); !errors.As(err, &bad) {
		t.Fatalf("got %v", err)
	}
}

func TestWriteFailsWhenTheDomainFolderCannotBeMade(t *testing.T) {
	folder, root := testFolder(t)
	os.WriteFile(filepath.Join(root, "example.com"), []byte("a file, not a folder"), 0o600)
	if err := folder.Write(Key{DomainId: "example.com", Name: "a1"}, strings.NewReader("x"), 1); err == nil {
		t.Fatal("expected an error")
	}
}

func TestWriteFailsWhenTheTemporaryFileCannotBeMade(t *testing.T) {
	if err := (AtomicFile{Path: filepath.Join(t.TempDir(), "missing", "a1")}).Write(strings.NewReader("x"), 1); err == nil {
		t.Fatal("expected an error")
	}
}

func TestReadOfAMissingBlobIsNotFound(t *testing.T) {
	folder, _ := testFolder(t)
	var missing NotFound
	if _, err := folder.Read(Key{DomainId: "example.com", Name: "nope"}); !errors.As(err, &missing) || missing.Path != "example.com/nope" {
		t.Fatalf("got %v", err)
	}
}

func TestReadRefusesABadKey(t *testing.T) {
	folder, _ := testFolder(t)
	if _, err := folder.Read(Key{DomainId: "../etc", Name: "passwd"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestReadPassesOnOtherOpenErrors(t *testing.T) {
	folder, root := testFolder(t)
	os.WriteFile(filepath.Join(root, "example.com"), []byte("a file, not a folder"), 0o600)
	var missing NotFound
	if _, err := folder.Read(Key{DomainId: "example.com", Name: "a1"}); err == nil || errors.As(err, &missing) {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteRemovesAndForgivesMissingBlobs(t *testing.T) {
	folder, _ := testFolder(t)
	key := Key{DomainId: "example.com", Name: "a1"}
	folder.Write(key, strings.NewReader("x"), 1)
	if err := folder.Delete(key); err != nil {
		t.Fatal(err)
	}
	if err := folder.Delete(key); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	if _, err := folder.Read(key); err == nil {
		t.Fatal("blob still there")
	}
}

func TestDeleteRefusesABadKey(t *testing.T) {
	folder, _ := testFolder(t)
	if err := folder.Delete(Key{DomainId: "example.com", Name: ".."}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDeletePassesOnOtherErrors(t *testing.T) {
	folder, root := testFolder(t)
	key := Key{DomainId: "example.com", Name: "a1"}
	os.MkdirAll(filepath.Join(root, "example.com", "a1", "child"), 0o700)
	if err := folder.Delete(key); err == nil {
		t.Fatal("expected an error for a folder that is not empty")
	}
}

func TestSignedUploadUrlGrantsAPutOfTheDeclaredSize(t *testing.T) {
	folder, _ := testFolder(t)
	signed, err := folder.SignUpload(Key{DomainId: "example.com", Name: "a1"}, UploadTerms{MediaType: "image/png", Size: 42}, testNow().Add(5*time.Minute))
	if err != nil || !strings.HasPrefix(signed, "http://127.0.0.1:8105/api/v1/attachments/blobs/example.com/a1?") {
		t.Fatalf("got %q, %v", signed, err)
	}
	grant, err := grantFromUrl(t, folder, http.MethodPut, signed, testNow())
	if err != nil || grant.Size != 42 || grant.MediaType != "image/png" || grant.Method != http.MethodPut {
		t.Fatalf("got %+v, %v", grant, err)
	}
}

func TestSignedDownloadUrlGrantsAGetWithTheFileName(t *testing.T) {
	folder, _ := testFolder(t)
	signed, _ := folder.SignDownload(Key{DomainId: "example.com", Name: "a1"}, DownloadTerms{FileName: "plan.pdf", MediaType: "application/pdf"}, testNow().Add(time.Minute))
	grant, err := grantFromUrl(t, folder, http.MethodGet, signed, testNow())
	if err != nil || grant.FileName != "plan.pdf" || grant.MediaType != "application/pdf" {
		t.Fatalf("got %+v, %v", grant, err)
	}
}

func TestADownloadGrantCannotBeUsedToUpload(t *testing.T) {
	folder, _ := testFolder(t)
	signed, _ := folder.SignDownload(Key{DomainId: "example.com", Name: "a1"}, DownloadTerms{FileName: "a", MediaType: "text/plain"}, testNow().Add(time.Minute))
	if _, err := grantFromUrl(t, folder, http.MethodPut, signed, testNow()); !errors.Is(err, BadSignature{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAGrantIsBoundToItsDomain(t *testing.T) {
	folder, _ := testFolder(t)
	signed, _ := folder.SignDownload(Key{DomainId: "example.com", Name: "a1"}, DownloadTerms{FileName: "a", MediaType: "text/plain"}, testNow().Add(time.Minute))
	parsed, _ := url.Parse(signed)
	if _, err := folder.Grant(http.MethodGet, Key{DomainId: "example.org", Name: "a1"}, parsed.Query(), testNow()); !errors.Is(err, BadSignature{}) {
		t.Fatalf("got %v", err)
	}
}

func TestATamperedGrantIsRefused(t *testing.T) {
	folder, _ := testFolder(t)
	signed, _ := folder.SignUpload(Key{DomainId: "example.com", Name: "a1"}, UploadTerms{MediaType: "image/png", Size: 42}, testNow().Add(time.Minute))
	tampered := strings.Replace(signed, "size=42", "size=999999", 1)
	if _, err := grantFromUrl(t, folder, http.MethodPut, tampered, testNow()); !errors.Is(err, BadSignature{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAnExpiredGrantIsRefused(t *testing.T) {
	folder, _ := testFolder(t)
	signed, _ := folder.SignUpload(Key{DomainId: "example.com", Name: "a1"}, UploadTerms{MediaType: "image/png", Size: 1}, testNow().Add(time.Minute))
	if _, err := grantFromUrl(t, folder, http.MethodPut, signed, testNow().Add(2*time.Minute)); !errors.Is(err, Expired{}) {
		t.Fatalf("got %v", err)
	}
}

func TestAGrantFromAnotherSecretIsRefused(t *testing.T) {
	folder, _ := testFolder(t)
	other := NewLocalFolder(t.TempDir(), "http://127.0.0.1:8105/api/v1/attachments/blobs", NewLocalSigner([]byte("other secret")))
	signed, _ := other.SignUpload(Key{DomainId: "example.com", Name: "a1"}, UploadTerms{MediaType: "image/png", Size: 1}, testNow().Add(time.Minute))
	if _, err := grantFromUrl(t, folder, http.MethodPut, signed, testNow()); !errors.Is(err, BadSignature{}) {
		t.Fatalf("got %v", err)
	}
}

func TestSigningAndGrantingRefuseBadKeys(t *testing.T) {
	folder, _ := testFolder(t)
	bad := Key{DomainId: "example.com", Name: "a/b"}
	_, uploadErr := folder.SignUpload(bad, UploadTerms{}, testNow())
	_, grantErr := folder.Grant(http.MethodGet, bad, url.Values{}, testNow())
	if uploadErr == nil || grantErr == nil {
		t.Fatalf("got %v, %v", uploadErr, grantErr)
	}
}
