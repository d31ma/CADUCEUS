import (
	"errors"
	"strings"
)

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type AttachmentsControllerTest struct {
	behaviors *BehaviorContext
}

func (test *AttachmentsControllerTest) uploaded(authorization string, domainId string, stored string) Upload {
	upload, _ := AttachmentsTestSetup{}.Controller().PostAttachment(authorization, AttachmentsTestSetup{}.Request("hello", "text/plain"))
	AttachmentsTestSetup{}.Store(domainId, upload.AttachmentId, stored)
	return upload
}

func (test *AttachmentsControllerTest) TestAMemberUploadsCompletesAndDownloads() {
	domainId := AttachmentsTestSetup{}.Domain()
	ana := AttachmentsTestSetup{}.SignedIn(domainId, "ana", "secret")
	upload := test.uploaded(ana, domainId, "hello")
	attachment, completeErr := AttachmentsTestSetup{}.Controller().PostComplete(ana, upload.AttachmentId)
	download, downloadErr := AttachmentsTestSetup{}.Controller().GetAttachment(ana, upload.AttachmentId)
	Assert.IsTrue(strings.HasPrefix(upload.UploadUrl, "http://127.0.0.1:8100/blobs/"+domainId+"/"), upload.UploadUrl)
	Assert.IsTrue(completeErr == nil, "complete failed")
	Assert.Equals("ready", attachment.State)
	Assert.Equals(int64(5), attachment.Size)
	Assert.IsTrue(downloadErr == nil, "download failed")
	Assert.IsTrue(strings.Contains(download.Url, "/"+domainId+"/"), download.Url)
}

func (test *AttachmentsControllerTest) TestAnotherMemberGetsNotFoundWithoutMessagingAccess() {
	domainId := AttachmentsTestSetup{}.Domain()
	ana := AttachmentsTestSetup{}.SignedIn(domainId, "ana", "secret")
	upload := test.uploaded(ana, domainId, "hello")
	AttachmentsTestSetup{}.Controller().PostComplete(ana, upload.AttachmentId)
	_, err := AttachmentsTestSetup{}.Controller().GetAttachment(AttachmentsTestSetup{}.SignedIn(domainId, "bo", "secret"), upload.AttachmentId)
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(err, &missing), "bo got a link without messaging access")
}

func (test *AttachmentsControllerTest) TestAMemberOfAnotherDomainGetsNotFound() {
	domainId := AttachmentsTestSetup{}.Domain()
	ana := AttachmentsTestSetup{}.SignedIn(domainId, "ana", "secret")
	upload := test.uploaded(ana, domainId, "hello")
	AttachmentsTestSetup{}.Controller().PostComplete(ana, upload.AttachmentId)
	_, err := AttachmentsTestSetup{}.Controller().GetAttachment(AttachmentsTestSetup{}.SignedIn(AttachmentsTestSetup{}.Domain(), "ana", "secret"), upload.AttachmentId)
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(err, &missing), "another domain got a link")
}

func (test *AttachmentsControllerTest) TestCompleteWithTheWrongBytesIsAConflict() {
	domainId := AttachmentsTestSetup{}.Domain()
	ana := AttachmentsTestSetup{}.SignedIn(domainId, "ana", "secret")
	upload := test.uploaded(ana, domainId, "jello")
	_, err := AttachmentsTestSetup{}.Controller().PostComplete(ana, upload.AttachmentId)
	conflict := &AttachmentConflict{}
	Assert.IsTrue(errors.As(err, &conflict), "wrong bytes accepted")
	Assert.Equals("sha256", conflict.Field)
	Assert.Equals(3510, conflict.Number())
}

func (test *AttachmentsControllerTest) TestPostAttachmentRefusesABadRequest() {
	ana := AttachmentsTestSetup{}.SignedIn(AttachmentsTestSetup{}.Domain(), "ana", "secret")
	request := AttachmentsTestSetup{}.Request("hello", "text/plain")
	request.FileName = "../escape.txt"
	_, err := AttachmentsTestSetup{}.Controller().PostAttachment(ana, request)
	invalid := &InvalidUpload{}
	Assert.IsTrue(errors.As(err, &invalid), "bad file name accepted")
	Assert.Equals("fileName", invalid.Field)
	Assert.Equals(3501, invalid.Number())
}

func (test *AttachmentsControllerTest) TestPostAttachmentRefusesAFileOverTheLimit() {
	ana := AttachmentsTestSetup{}.SignedIn(AttachmentsTestSetup{}.Domain(), "ana", "secret")
	_, err := AttachmentsTestSetup{}.Controller().PostAttachment(ana, AttachmentsTestSetup{}.Request(strings.Repeat("x", 101), "text/plain"))
	large := &AttachmentTooLarge{}
	Assert.IsTrue(errors.As(err, &large), "large file accepted")
	Assert.Equals(int64(100), large.Limit)
}

func (test *AttachmentsControllerTest) TestPostAttachmentFailsClosedWithoutAValidToken() {
	AttachmentsTestSetup{}.SignedIn(AttachmentsTestSetup{}.Domain(), "ana", "secret")
	_, err := AttachmentsTestSetup{}.Controller().PostAttachment("Bearer forged", AttachmentsTestSetup{}.Request("hello", "text/plain"))
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused), "a forged token was accepted")
}

func (test *AttachmentsControllerTest) TestStorageWithoutALinkSecretIsUnavailable() {
	ana := AttachmentsTestSetup{}.SignedIn(AttachmentsTestSetup{}.Domain(), "ana", "")
	_, err := AttachmentsTestSetup{}.Controller().PostAttachment(ana, AttachmentsTestSetup{}.Request("hello", "text/plain"))
	unavailable := &StorageUnavailable{}
	Assert.IsTrue(errors.As(err, &unavailable), "signed without a secret")
}
