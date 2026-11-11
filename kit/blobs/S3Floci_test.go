package blobs

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func flociStore(t *testing.T) S3Store {
	endpoint := os.Getenv("MESAIL_S3_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("MESAIL_S3_TEST_ENDPOINT is not set")
	}
	settings := S3Settings{Endpoint: endpoint, Region: "us-east-1", Bucket: "mesail-slice-a1-test", AccessKeyId: "test", SecretAccessKey: "test"}
	return NewS3Store(settings, &http.Client{Timeout: 10 * time.Second}, time.Now)
}

func TestFlociStoresReadsAndDeletesThroughSignedUrls(t *testing.T) {
	store := flociStore(t)
	key := Key{DomainId: "example.com", Name: "floci-check"}
	if err := store.EnsureBucket(); err != nil {
		t.Fatal(err)
	}
	upload, _ := store.SignUpload(key, UploadTerms{MediaType: "text/plain", Size: 5}, time.Now().Add(time.Minute))
	request, _ := http.NewRequest(http.MethodPut, upload, strings.NewReader("hello"))
	response, err := http.DefaultClient.Do(request)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("upload: %v %v", err, response)
	}
	reader, err := store.Read(key)
	if err != nil {
		t.Fatal(err)
	}
	inspection, _ := Inspector{}.Inspect(reader, 5)
	reader.Close()
	if inspection.Sha256 != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("got %+v", inspection)
	}
	download, _ := store.SignDownload(key, DownloadTerms{FileName: "hello.txt", MediaType: "text/plain"}, time.Now().Add(time.Minute))
	response, err = http.Get(download)
	if err != nil || response.StatusCode != http.StatusOK || !strings.Contains(response.Header.Get("Content-Disposition"), "hello.txt") {
		t.Fatalf("download: %v %v", err, response)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "hello" {
		t.Fatalf("got %q", body)
	}
	copied := Key{DomainId: "example.org", Name: "floci-check"}
	if err := store.Copy(key, copied); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if err := store.Write(copied, strings.NewReader("replaced"), 8); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := store.Delete(key); err != nil {
		t.Fatal(err)
	}
	store.Delete(copied)
	var missing NotFound
	if _, err := store.Read(key); !errors.As(err, &missing) {
		t.Fatalf("after delete: %v", err)
	}
}
