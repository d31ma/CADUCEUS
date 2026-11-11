import (
	"errors"
	"strings"
	"time"

	"mesail.dev/packages/blobs"
)

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type BlobStoreServiceTest struct {
	behaviors *BehaviorContext
}

func (test *BlobStoreServiceTest) TestStoreSignsLocalLinksUnderTheLinkBase() {
	store, err := (&BlobStoreService{}).Store(AttachmentSettings{Adapter: "local", Folder: "/tmp", Link: LinkSettings{Base: "http://127.0.0.1:8100/blobs", Secret: "s"}})
	signed, _ := store.SignUpload(blobs.Key{DomainId: "example.com", Name: "a1"}, blobs.UploadTerms{MediaType: "text/plain", Size: 1}, time.Now().Add(time.Minute))
	Assert.IsTrue(err == nil, "store refused")
	Assert.IsTrue(strings.HasPrefix(signed, "http://127.0.0.1:8100/blobs/example.com/a1?"), signed)
}

func (test *BlobStoreServiceTest) TestStoreSignsS3LinksForTheBucket() {
	settings := AttachmentSettings{Adapter: "s3", Bucket: "mesail-attachments", Region: "us-east-1", Endpoint: "http://floci:4566", Public: "http://127.0.0.1:14566"}
	store, err := (&BlobStoreService{}).Store(settings)
	signed, _ := store.SignDownload(blobs.Key{DomainId: "example.com", Name: "a1"}, blobs.DownloadTerms{FileName: "a.txt", MediaType: "text/plain"}, time.Now().Add(time.Minute))
	Assert.IsTrue(err == nil, "store refused")
	Assert.IsTrue(strings.HasPrefix(signed, "http://127.0.0.1:14566/mesail-attachments/example.com/a1?"), signed)
	Assert.IsTrue(strings.Contains(signed, "X-Amz-Signature="), signed)
}

func (test *BlobStoreServiceTest) TestStoreRefusesLocalLinksWithoutASecret() {
	_, err := (&BlobStoreService{}).Store(AttachmentSettings{Adapter: "local", Folder: "/tmp"})
	unavailable := &StorageUnavailable{}
	Assert.IsTrue(errors.As(err, &unavailable), "store without a secret was accepted")
}
