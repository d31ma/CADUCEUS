package adapters

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/blobs"
)

type recordedPut struct {
	method        string
	path          string
	authorization string
	body          string
}

func localBlobStore(t *testing.T) blobs.LocalFolder {
	return blobs.NewLocalFolder(t.TempDir(), "", blobs.NewLocalSigner(nil))
}

func fakeS3(t *testing.T, status int) (blobs.S3Store, *[]recordedPut) {
	seen := &[]recordedPut{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		sent, _ := io.ReadAll(request.Body)
		*seen = append(*seen, recordedPut{method: request.Method, path: request.URL.Path, authorization: request.Header.Get("Authorization"), body: string(sent)})
		writer.WriteHeader(status)
	}))
	t.Cleanup(server.Close)
	settings := blobs.S3Settings{Endpoint: server.URL, Region: "us-east-1", Bucket: "mesail-raw", AccessKeyId: "test", SecretAccessKey: "test"}
	now := func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC) }
	return blobs.NewS3Store(settings, server.Client(), now), seen
}

func TestBlobRawStoreWritesTheContentUnderTheKey(t *testing.T) {
	folder := localBlobStore(t)
	err := NewBlobRawStore(folder).Put(context.Background(), "example.com/inbound-abc", []byte("raw"))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := folder.Read(blobs.Key{DomainId: "example.com", Name: "inbound-abc"})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	content, _ := io.ReadAll(reader)
	if string(content) != "raw" {
		t.Fatalf("unexpected content %q", content)
	}
}

func TestBlobRawStoreOverwritesTheSameKey(t *testing.T) {
	folder := localBlobStore(t)
	store := NewBlobRawStore(folder)
	store.Put(context.Background(), "example.com/inbound-abc", []byte("first"))
	store.Put(context.Background(), "example.com/inbound-abc", []byte("second"))
	reader, _ := folder.Read(blobs.Key{DomainId: "example.com", Name: "inbound-abc"})
	defer reader.Close()
	content, _ := io.ReadAll(reader)
	if string(content) != "second" {
		t.Fatalf("want the second copy, got %q", content)
	}
}

func TestBlobRawStoreRefusesParentFolderKeys(t *testing.T) {
	err := NewBlobRawStore(localBlobStore(t)).Put(context.Background(), "example.com/../../etc/passwd", []byte("x"))
	if !errors.Is(err, InvalidKey{}) || err.Error() == "" {
		t.Fatalf("want InvalidKey, got %v", err)
	}
}

func TestBlobRawStoreRefusesKeysWithoutAName(t *testing.T) {
	err := NewBlobRawStore(localBlobStore(t)).Put(context.Background(), "example.com", []byte("x"))
	if !errors.Is(err, InvalidKey{}) {
		t.Fatalf("want InvalidKey, got %v", err)
	}
}

func TestBlobRawStoreRefusesBadDomains(t *testing.T) {
	err := NewBlobRawStore(localBlobStore(t)).Put(context.Background(), "/etc/passwd", []byte("x"))
	if !errors.Is(err, InvalidKey{}) {
		t.Fatalf("want InvalidKey, got %v", err)
	}
}

func TestBlobRawStoreRefusesBadKeysBeforeAnyRequest(t *testing.T) {
	store, seen := fakeS3(t, http.StatusOK)
	err := NewBlobRawStore(store).Put(context.Background(), "example.com/a/b", []byte("x"))
	if !errors.Is(err, InvalidKey{}) || len(*seen) != 0 {
		t.Fatalf("want InvalidKey and no request, got %v and %d requests", err, len(*seen))
	}
}

func TestBlobRawStoreSendsASignedPutToS3(t *testing.T) {
	store, seen := fakeS3(t, http.StatusOK)
	err := NewBlobRawStore(store).Put(context.Background(), "example.com/inbound-abc", []byte("raw"))
	if err != nil || len(*seen) != 1 {
		t.Fatalf("want one request, got %v and %d", err, len(*seen))
	}
	put := (*seen)[0]
	if put.method != http.MethodPut || put.path != "/mesail-raw/example.com/inbound-abc" || put.body != "raw" {
		t.Fatalf("unexpected request %+v", put)
	}
	if !strings.HasPrefix(put.authorization, "AWS4-HMAC-SHA256 Credential=test/20261111/us-east-1/s3/aws4_request") || !strings.Contains(put.authorization, "Signature=") {
		t.Fatalf("unexpected authorization %q", put.authorization)
	}
}

func TestBlobRawStoreReportsAnS3Failure(t *testing.T) {
	store, _ := fakeS3(t, http.StatusInternalServerError)
	err := NewBlobRawStore(store).Put(context.Background(), "example.com/inbound-abc", []byte("raw"))
	if err == nil {
		t.Fatal("want an error")
	}
}
