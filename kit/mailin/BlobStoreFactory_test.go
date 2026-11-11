package mailin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/blobs"
)

func TestALocalStoreNeedsAFolderAndASecret(t *testing.T) {
	store := BlobStoreFactory{Environment: testEnvironment(nil)}.Store(BlobSettings{Adapter: "local", Folder: t.TempDir()})
	_, isMissing := store.(MissingBlobs)
	check(t, isMissing, "want missing blobs")
}

func TestALocalStoreWritesIntoItsFolder(t *testing.T) {
	settings := BlobSettings{Adapter: "local", Folder: t.TempDir(), Link: LinkSettings{Base: "https://mesail.test/blobs", Secret: "fixture-link-value"}}
	store := BlobStoreFactory{Environment: testEnvironment(nil)}.Store(settings)
	key := blobs.Key{DomainId: "example.com", Name: "inbound-1"}
	err := store.Write(key, strings.NewReader("raw"), 3)
	url, signErr := store.SignDownload(key, MessageDesk{}.OriginalTerms(), fixedNow())
	check(t, err == nil && signErr == nil, "local store failed")
	check(t, strings.HasPrefix(url, "https://mesail.test/blobs/example.com/inbound-1?"), "wrong link")
}

func TestAnS3StoreUsesTheBucket(t *testing.T) {
	settings := BlobSettings{Adapter: "s3", Bucket: "mesail-raw", Region: "us-east-1", Endpoint: "http://floci.test:4566"}
	store := BlobStoreFactory{Environment: testEnvironment(map[string]string{"AWS_ACCESS_KEY_ID": "id", "AWS_SECRET_ACCESS_KEY": "key"})}.Store(settings)
	url, err := store.SignDownload(blobs.Key{DomainId: "example.com", Name: "inbound-1"}, MessageDesk{}.OriginalTerms(), time.Now().Add(time.Minute))
	check(t, err == nil && strings.HasPrefix(url, "http://floci.test:4566/mesail-raw/example.com/inbound-1?"), "wrong link")
}

func TestAnS3StoreClosesEachConnection(t *testing.T) {
	closed := make(chan bool, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		closed <- request.Close
		writer.Write([]byte("raw"))
	}))
	defer server.Close()
	settings := BlobSettings{Adapter: "s3", Bucket: "mesail-raw", Region: "us-east-1", Endpoint: server.URL}
	store := BlobStoreFactory{Environment: testEnvironment(map[string]string{"AWS_ACCESS_KEY_ID": "id", "AWS_SECRET_ACCESS_KEY": "key"})}.Store(settings)
	reader, err := store.Read(blobs.Key{DomainId: "example.com", Name: "inbound-1"})
	check(t, err == nil && <-closed, "the S3 client kept the connection alive")
	reader.Close()
}

func TestMissingBlobsRefuseEveryCall(t *testing.T) {
	missing, key := MissingBlobs{}, blobs.Key{DomainId: "example.com", Name: "inbound-1"}
	_, upload := missing.SignUpload(key, blobs.UploadTerms{}, fixedNow())
	_, download := missing.SignDownload(key, blobs.DownloadTerms{}, fixedNow())
	_, read := missing.Read(key)
	check(t, isUnavailable(upload, "blob store settings") && isUnavailable(download, "blob store settings"), "want unavailable")
	check(t, isUnavailable(read, "blob store settings"), "want unavailable")
	check(t, isUnavailable(missing.Write(key, strings.NewReader(""), 0), "blob store settings"), "want unavailable")
	check(t, isUnavailable(missing.Copy(key, key), "blob store settings"), "want unavailable")
	check(t, isUnavailable(missing.Delete(key), "blob store settings"), "want unavailable")
}
