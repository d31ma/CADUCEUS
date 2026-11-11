package blobs

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testHandler(t *testing.T) (LocalHandler, LocalFolder) {
	folder := NewLocalFolder(t.TempDir(), "http://127.0.0.1:8100/blobs", NewLocalSigner([]byte("test secret")))
	return NewLocalHandler(folder, "/blobs", testNow), folder
}

func serve(handler LocalHandler, method string, target string, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, target, strings.NewReader(body)))
	return recorder
}

func TestHandlerStoresAnUploadThenServesTheDownload(t *testing.T) {
	handler, folder := testHandler(t)
	key := Key{DomainId: "example.com", Name: "a1"}
	upload, _ := folder.SignUpload(key, UploadTerms{MediaType: "text/plain", Size: 5}, testNow().Add(time.Minute))
	if response := serve(handler, http.MethodPut, upload, "hello"); response.Code != http.StatusOK {
		t.Fatalf("upload answered %d", response.Code)
	}
	download, _ := folder.SignDownload(key, DownloadTerms{FileName: "hello.txt", MediaType: "text/plain"}, testNow().Add(time.Minute))
	response := serve(handler, http.MethodGet, download, "")
	body, _ := io.ReadAll(response.Body)
	if response.Code != http.StatusOK || string(body) != "hello" {
		t.Fatalf("download answered %d %q", response.Code, body)
	}
	header := response.Header()
	if header.Get("Content-Type") != "text/plain" || header.Get("Content-Disposition") != "attachment; filename=hello.txt" || header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers %v", header)
	}
}

func TestHandlerRefusesAnUploadOfTheWrongSize(t *testing.T) {
	handler, folder := testHandler(t)
	upload, _ := folder.SignUpload(Key{DomainId: "example.com", Name: "a1"}, UploadTerms{MediaType: "text/plain", Size: 5}, testNow().Add(time.Minute))
	if response := serve(handler, http.MethodPut, upload, "hello world"); response.Code != http.StatusBadRequest {
		t.Fatalf("answered %d", response.Code)
	}
}

func TestHandlerRefusesExpiredAndForgedLinks(t *testing.T) {
	handler, folder := testHandler(t)
	key := Key{DomainId: "example.com", Name: "a1"}
	expired, _ := folder.SignUpload(key, UploadTerms{MediaType: "text/plain", Size: 1}, testNow().Add(-time.Minute))
	download, _ := folder.SignDownload(key, DownloadTerms{FileName: "a", MediaType: "text/plain"}, testNow().Add(time.Minute))
	responses := []*httptest.ResponseRecorder{
		serve(handler, http.MethodPut, expired, "x"),
		serve(handler, http.MethodPut, download, "x"),
		serve(handler, http.MethodDelete, download, ""),
		serve(handler, http.MethodGet, strings.Replace(download, "example.com", "example.org", 1), ""),
	}
	for index, response := range responses {
		if response.Code != http.StatusForbidden {
			t.Fatalf("case %d answered %d", index, response.Code)
		}
	}
}

func TestHandlerAnswersNotFoundForMissingBlobsAndBadKeys(t *testing.T) {
	handler, folder := testHandler(t)
	download, _ := folder.SignDownload(Key{DomainId: "example.com", Name: "nope"}, DownloadTerms{FileName: "a", MediaType: "text/plain"}, testNow().Add(time.Minute))
	missing := serve(handler, http.MethodGet, download, "")
	badKey := serve(handler, http.MethodGet, "/blobs/example.com/a.b?signature=x", "")
	if missing.Code != http.StatusNotFound || badKey.Code != http.StatusNotFound {
		t.Fatalf("answered %d and %d", missing.Code, badKey.Code)
	}
}

func TestStatusOfAnUnknownErrorIsServerError(t *testing.T) {
	if got := (LocalHandler{}).status(errors.New("disk gone")); got != http.StatusInternalServerError {
		t.Fatalf("got %d", got)
	}
}

func TestErrorsNameTheirHttpStatus(t *testing.T) {
	statuses := []int{
		BadSignature{}.HttpStatus(), Expired{}.HttpStatus(), BadKey{}.HttpStatus(), NotFound{}.HttpStatus(),
		SizeMismatch{}.HttpStatus(), TooLarge{}.HttpStatus(), S3Failure{}.HttpStatus(),
	}
	wanted := []int{403, 403, 404, 404, 400, 413, 502}
	for index, status := range statuses {
		if status != wanted[index] {
			t.Fatalf("case %d: got %d", index, status)
		}
	}
}
