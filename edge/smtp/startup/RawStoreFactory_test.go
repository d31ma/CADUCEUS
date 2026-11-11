package startup

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type seenPuts struct {
	mutex          sync.Mutex
	paths          []string
	authorizations []string
}

func (seen *seenPuts) record(request *http.Request) {
	seen.mutex.Lock()
	defer seen.mutex.Unlock()
	seen.paths = append(seen.paths, request.Method+" "+request.URL.Path)
	seen.authorizations = append(seen.authorizations, request.Header.Get("Authorization"))
}

func fakeS3Server(t *testing.T) (*httptest.Server, *seenPuts) {
	seen := &seenPuts{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		io.Copy(io.Discard, request.Body)
		seen.record(request)
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server, seen
}

func secretFile(t *testing.T, content string) string {
	file := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(file, []byte(content), 0o600)
	return file
}

func s3Settings(t *testing.T, endpoint string, secret string) ListenerSettings {
	settings := baseSettings(t)
	settings.RawStoreS3 = S3RawStoreSettings{Endpoint: endpoint, Bucket: "mesail-raw", Region: "eu-west-1", AccessKeyId: "edge-key", SecretFile: secret}
	return settings
}

func TestRawStoreFactoryUsesTheLocalFolderWithoutABucket(t *testing.T) {
	settings := baseSettings(t)
	store, err := RawStoreFactory{}.Store(settings)
	if err != nil {
		t.Fatal(err)
	}
	err = store.Put(context.Background(), "example.com/inbound-abc", []byte("raw"))
	content, readError := os.ReadFile(filepath.Join(settings.RawStoreFolder, "example.com", "inbound-abc"))
	if err != nil || readError != nil || string(content) != "raw" {
		t.Fatalf("unexpected result %q %v %v", content, err, readError)
	}
}

func TestRawStoreFactoryUsesS3WithABucket(t *testing.T) {
	server, seen := fakeS3Server(t)
	settings := s3Settings(t, server.URL, secretFile(t, "edge-secret\n"))
	store, err := RawStoreFactory{}.Store(settings)
	if err != nil {
		t.Fatal(err)
	}
	err = store.Put(context.Background(), "example.com/inbound-abc", []byte("raw"))
	if err != nil || len(seen.paths) != 1 || seen.paths[0] != "PUT /mesail-raw/example.com/inbound-abc" {
		t.Fatalf("unexpected requests %v %v", seen.paths, err)
	}
	if !strings.Contains(seen.authorizations[0], "Credential=edge-key/") || !strings.Contains(seen.authorizations[0], "/eu-west-1/s3/aws4_request") {
		t.Fatalf("unexpected authorization %q", seen.authorizations[0])
	}
	if _, statError := os.Stat(filepath.Join(settings.RawStoreFolder, "example.com")); statError == nil {
		t.Fatal("the local folder must not be used")
	}
}

func TestRawStoreFactoryRefusesAMissingSecretFile(t *testing.T) {
	settings := s3Settings(t, "http://s3.local", filepath.Join(t.TempDir(), "missing"))
	_, err := RawStoreFactory{}.Store(settings)
	var invalid InvalidSetting
	if !errors.As(err, &invalid) || invalid.Name != "MESAIL_RAW_STORE_S3_SECRET_ACCESS_KEY_FILE" {
		t.Fatalf("want the secret file to be named, got %v", err)
	}
}

func TestRawStoreFactoryRefusesAnEmptySecretFile(t *testing.T) {
	settings := s3Settings(t, "http://s3.local", secretFile(t, "  \n"))
	_, err := RawStoreFactory{}.Store(settings)
	if err == nil || !strings.Contains(err.Error(), "MESAIL_RAW_STORE_S3_SECRET_ACCESS_KEY_FILE") {
		t.Fatalf("want the secret file to be named, got %v", err)
	}
}

func TestWiringFailsAtStartupWhenTheS3SecretIsMissing(t *testing.T) {
	settings := s3Settings(t, "http://s3.local", filepath.Join(t.TempDir(), "missing"))
	if _, err := (Wiring{}).Build(settings, nil); err == nil {
		t.Fatal("want an error")
	}
}

func TestRawStoreFactoryGivesS3AFiveMinuteUploadTimeoutByDefault(t *testing.T) {
	client := RawStoreFactory{}.uploadClient(S3RawStoreSettings{})
	if client.Timeout != 5*time.Minute {
		t.Fatalf("want 5 minutes, got %v", client.Timeout)
	}
}

func TestRawStoreFactoryGivesS3TheConfiguredUploadTimeout(t *testing.T) {
	client := RawStoreFactory{}.uploadClient(S3RawStoreSettings{Timeout: 42 * time.Second})
	if client.Timeout != 42*time.Second {
		t.Fatalf("want 42 seconds, got %v", client.Timeout)
	}
}

func TestRawStoreFactoryS3UploadsStopAtTheTimeout(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		<-release
	}))
	t.Cleanup(server.Close)
	t.Cleanup(func() { close(release) })
	settings := s3Settings(t, server.URL, secretFile(t, "edge-secret"))
	settings.RawStoreS3.Timeout = 50 * time.Millisecond
	store, err := RawStoreFactory{}.Store(settings)
	if err != nil {
		t.Fatal(err)
	}
	err = store.Put(context.Background(), "example.com/inbound-abc", []byte("raw"))
	if err == nil || !strings.Contains(err.Error(), "Timeout") {
		t.Fatalf("want a timeout, got %v", err)
	}
}
