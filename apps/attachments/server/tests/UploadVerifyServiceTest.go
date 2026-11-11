import (
	"errors"
	"strings"

	"mesail.dev/packages/blobs"
)

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type UploadVerifyServiceTest struct {
	behaviors *BehaviorContext
}

func (test *UploadVerifyServiceTest) verify(declared CreateUploadRequest, content string) (string, error) {
	settings := AttachmentsTestSetup{}.Settings("")
	key := blobs.Key{DomainId: "example.com", Name: "VERIFY1"}
	AttachmentsTestSetup{}.Folder(settings).Write(key, strings.NewReader(content), int64(len(content)))
	store, _ := (&BlobStoreService{}).Store(settings)
	record := AttachmentRecord{MediaType: declared.MediaType, Size: declared.Size, Sha256: declared.Sha256}
	return (&UploadVerifyService{}).Verify(store, key, record)
}

func (test *UploadVerifyServiceTest) conflictField(err error) string {
	conflict := &AttachmentConflict{}
	errors.As(err, &conflict)
	return conflict.Field
}

func (test *UploadVerifyServiceTest) TestVerifyMarksAMatchingFileReady() {
	state, err := test.verify(AttachmentsTestSetup{}.Request("hello", "text/plain"), "hello")
	Assert.IsTrue(err == nil, "verify failed")
	Assert.Equals("ready", state)
}

func (test *UploadVerifyServiceTest) TestVerifyBlocksHtmlDeclaredAsAnImage() {
	content := "<!DOCTYPE html><html><script>alert(1)</script></html>"
	state, err := test.verify(AttachmentsTestSetup{}.Request(content, "image/png"), content)
	Assert.IsTrue(err == nil, "verify failed")
	Assert.Equals("blocked", state)
}

func (test *UploadVerifyServiceTest) TestVerifyRefusesTheWrongSha256() {
	declared := AttachmentsTestSetup{}.Request("hello", "text/plain")
	_, err := test.verify(declared, "jello")
	Assert.Equals("sha256", test.conflictField(err))
}

func (test *UploadVerifyServiceTest) TestVerifyRefusesASmallerFile() {
	declared := AttachmentsTestSetup{}.Request("hello", "text/plain")
	_, err := test.verify(declared, "hell")
	Assert.Equals("size", test.conflictField(err))
}

func (test *UploadVerifyServiceTest) TestVerifyRefusesALargerFile() {
	declared := AttachmentsTestSetup{}.Request("hello", "text/plain")
	_, err := test.verify(declared, "hello world")
	large := &AttachmentTooLarge{}
	Assert.IsTrue(errors.As(err, &large), "larger file accepted")
}

func (test *UploadVerifyServiceTest) TestVerifyOfAMissingFileIsAConflict() {
	settings := AttachmentsTestSetup{}.Settings("")
	store, _ := (&BlobStoreService{}).Store(settings)
	_, err := (&UploadVerifyService{}).Verify(store, blobs.Key{DomainId: "example.com", Name: "NOTHERE"}, AttachmentRecord{Size: 1})
	Assert.Equals("file", test.conflictField(err))
}

func (test *UploadVerifyServiceTest) TestVerifyReportsAStoreThatCannotBeRead() {
	store, _ := (&BlobStoreService{}).Store(AttachmentSettings{Adapter: "local", Folder: "/tmp", Link: LinkSettings{Secret: "s"}})
	_, err := (&UploadVerifyService{}).Verify(store, blobs.Key{DomainId: "example.com", Name: "../bad"}, AttachmentRecord{Size: 1})
	unavailable := &StorageUnavailable{}
	Assert.IsTrue(errors.As(err, &unavailable), "bad key not reported")
}
