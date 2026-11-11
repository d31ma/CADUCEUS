package blobs

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type recordedRequest struct {
	method        string
	path          string
	authorization string
}

func fakeS3(t *testing.T, status int, body string) (S3Store, *[]recordedRequest) {
	seen := &[]recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		*seen = append(*seen, recordedRequest{method: request.Method, path: request.URL.Path, authorization: request.Header.Get("Authorization")})
		writer.WriteHeader(status)
		io.WriteString(writer, body)
	}))
	t.Cleanup(server.Close)
	settings := S3Settings{Endpoint: server.URL, Region: "us-east-1", Bucket: "mesail-attachments", AccessKeyId: "test", SecretAccessKey: "test"}
	return NewS3Store(settings, server.Client(), testNow), seen
}

func TestS3SignUploadPresignsAPutOfTheExactSize(t *testing.T) {
	store, _ := fakeS3(t, http.StatusOK, "")
	signed, err := store.SignUpload(Key{DomainId: "example.com", Name: "a1"}, UploadTerms{MediaType: "image/png", Size: 42}, testNow().Add(5*time.Minute))
	parsed, _ := url.Parse(signed)
	query := parsed.Query()
	if err != nil || parsed.Path != "/mesail-attachments/example.com/a1" || query.Get("X-Amz-Expires") != "300" {
		t.Fatalf("got %s, %v", signed, err)
	}
	if query.Get("X-Amz-SignedHeaders") != "content-length;host" || query.Get("X-Amz-Signature") == "" {
		t.Fatalf("got %s", signed)
	}
}

func TestS3SignDownloadAsksForAnAttachmentDisposition(t *testing.T) {
	store, _ := fakeS3(t, http.StatusOK, "")
	signed, err := store.SignDownload(Key{DomainId: "example.com", Name: "a1"}, DownloadTerms{FileName: "plan.pdf", MediaType: "application/pdf"}, testNow().Add(time.Minute))
	query, _ := url.ParseQuery(strings.SplitN(signed, "?", 2)[1])
	if err != nil || query.Get("response-content-disposition") != `attachment; filename=plan.pdf` || query.Get("response-content-type") != "application/pdf" {
		t.Fatalf("got %s, %v", signed, err)
	}
	if query.Get("X-Amz-Expires") != "60" {
		t.Fatalf("got %s", signed)
	}
}

func TestS3SigningRefusesBadKeys(t *testing.T) {
	store, _ := fakeS3(t, http.StatusOK, "")
	bad := Key{DomainId: "example.com", Name: "../x"}
	_, uploadErr := store.SignUpload(bad, UploadTerms{}, testNow())
	_, downloadErr := store.SignDownload(bad, DownloadTerms{}, testNow())
	if uploadErr == nil || downloadErr == nil {
		t.Fatalf("got %v, %v", uploadErr, downloadErr)
	}
}

func TestS3ReadReturnsTheSignedObjectBody(t *testing.T) {
	store, seen := fakeS3(t, http.StatusOK, "hello")
	reader, err := store.Read(Key{DomainId: "example.com", Name: "a1"})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(reader)
	reader.Close()
	request := (*seen)[0]
	if string(body) != "hello" || request.method != http.MethodGet || request.path != "/mesail-attachments/example.com/a1" {
		t.Fatalf("got %q, %+v", body, request)
	}
	if !strings.HasPrefix(request.authorization, "AWS4-HMAC-SHA256 Credential=test/20261110/us-east-1/s3/aws4_request") {
		t.Fatalf("got %s", request.authorization)
	}
}

func TestS3ReadOfAMissingObjectIsNotFound(t *testing.T) {
	store, _ := fakeS3(t, http.StatusNotFound, "<Error/>")
	var missing NotFound
	if _, err := store.Read(Key{DomainId: "example.com", Name: "a1"}); !errors.As(err, &missing) {
		t.Fatalf("got %v", err)
	}
}

func TestS3ReadReportsOtherStatuses(t *testing.T) {
	store, _ := fakeS3(t, http.StatusForbidden, "<Error/>")
	var failure S3Failure
	if _, err := store.Read(Key{DomainId: "example.com", Name: "a1"}); !errors.As(err, &failure) || failure.Status != http.StatusForbidden {
		t.Fatalf("got %v", err)
	}
}

func TestS3ReadRefusesBadKeys(t *testing.T) {
	store, seen := fakeS3(t, http.StatusOK, "")
	if _, err := store.Read(Key{DomainId: "Example.com", Name: "a1"}); err == nil || len(*seen) != 0 {
		t.Fatalf("got %v after %d requests", err, len(*seen))
	}
}

func TestS3DeleteSendsADeleteAndForgivesMissingObjects(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusNotFound} {
		store, seen := fakeS3(t, status, "")
		if err := store.Delete(Key{DomainId: "example.com", Name: "a1"}); err != nil || (*seen)[0].method != http.MethodDelete {
			t.Fatalf("status %d: got %v", status, err)
		}
	}
}

func TestS3DeleteReportsOtherStatuses(t *testing.T) {
	store, _ := fakeS3(t, http.StatusInternalServerError, "")
	if err := store.Delete(Key{DomainId: "example.com", Name: "a1"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestS3DeleteRefusesBadKeys(t *testing.T) {
	store, _ := fakeS3(t, http.StatusNoContent, "")
	if err := store.Delete(Key{DomainId: "example.com", Name: ""}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestS3RequestsFailWhenTheEndpointIsUnreachable(t *testing.T) {
	settings := S3Settings{Endpoint: "http://127.0.0.1:1", Region: "us-east-1", Bucket: "b", AccessKeyId: "test", SecretAccessKey: "test"}
	store := NewS3Store(settings, &http.Client{Timeout: time.Second}, testNow)
	_, readErr := store.Read(Key{DomainId: "example.com", Name: "a1"})
	deleteErr := store.Delete(Key{DomainId: "example.com", Name: "a1"})
	if readErr == nil || deleteErr == nil {
		t.Fatalf("got %v, %v", readErr, deleteErr)
	}
}

func TestS3RequestsFailWithABadEndpoint(t *testing.T) {
	store := NewS3Store(S3Settings{Endpoint: "http://bad host", Bucket: "b"}, http.DefaultClient, testNow)
	_, signErr := store.SignUpload(Key{DomainId: "example.com", Name: "a1"}, UploadTerms{}, testNow())
	_, readErr := store.Read(Key{DomainId: "example.com", Name: "a1"})
	if signErr == nil || readErr == nil {
		t.Fatalf("got %v, %v", signErr, readErr)
	}
}

func TestS3EnsureBucketCreatesTheBucketOnce(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusConflict} {
		store, seen := fakeS3(t, status, "")
		if err := store.EnsureBucket(); err != nil || (*seen)[0].method != http.MethodPut || (*seen)[0].path != "/mesail-attachments" {
			t.Fatalf("status %d: got %v, %+v", status, err, *seen)
		}
	}
}

func TestS3EnsureBucketReportsFailures(t *testing.T) {
	store, _ := fakeS3(t, http.StatusForbidden, "")
	unreachable := NewS3Store(S3Settings{Endpoint: "http://127.0.0.1:1", Bucket: "b"}, &http.Client{Timeout: time.Second}, testNow)
	badUrl := NewS3Store(S3Settings{Endpoint: "http://bad host", Bucket: "b"}, http.DefaultClient, testNow)
	if store.EnsureBucket() == nil || unreachable.EnsureBucket() == nil || badUrl.EnsureBucket() == nil {
		t.Fatal("expected errors")
	}
}
