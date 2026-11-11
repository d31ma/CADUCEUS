import (
	"errors"
	"strings"
)

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type AttachmentDownloadServiceTest struct {
	behaviors *BehaviorContext
}

func (test *AttachmentDownloadServiceTest) ready(domainId string) Upload {
	caller := AttachmentsTestSetup{}.Member(domainId, "ana")
	upload, _ := AttachmentsTestSetup{}.Uploaded(caller, "hello", "text/plain")
	AttachmentsTestSetup{}.Completer().complete(caller, upload.AttachmentId)
	return upload
}

func (test *AttachmentDownloadServiceTest) TestDownloadRefusesACallerWithoutAToken() {
	_, err := AttachmentsTestSetup{}.Downloader().Download("", "att_1")
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused), "no token was accepted")
}

func (test *AttachmentDownloadServiceTest) TestDownloadSignsALinkForTheUploader() {
	domainId := AttachmentsTestSetup{}.Domain()
	upload := test.ready(domainId)
	download, err := AttachmentsTestSetup{}.Downloader().download(AttachmentsTestSetup{}.Member(domainId, "ana"), AccessRequest{AttachmentId: upload.AttachmentId})
	Assert.IsTrue(err == nil, "download failed")
	Assert.IsTrue(strings.HasPrefix(download.Url, "http://127.0.0.1:8100/blobs/"+domainId+"/"), download.Url)
	Assert.IsTrue(strings.Contains(download.Url, "file=plan.txt"), download.Url)
	Assert.IsTrue(download.ExpiresAt != "", "no expiry")
}

func (test *AttachmentDownloadServiceTest) TestDownloadByAMemberWithoutAccessIsNotFound() {
	domainId := AttachmentsTestSetup{}.Domain()
	upload := test.ready(domainId)
	_, err := AttachmentsTestSetup{}.Downloader().download(AttachmentsTestSetup{}.Member(domainId, "bo"), AccessRequest{AttachmentId: upload.AttachmentId})
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(err, &missing), "a member without access got a link")
}

func (test *AttachmentDownloadServiceTest) TestDownloadFromAnotherDomainIsNotFound() {
	upload := test.ready(AttachmentsTestSetup{}.Domain())
	_, err := AttachmentsTestSetup{}.Downloader().download(AttachmentsTestSetup{}.Member(AttachmentsTestSetup{}.Domain(), "ana"), AccessRequest{AttachmentId: upload.AttachmentId})
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(err, &missing), "another domain got a link")
}

func (test *AttachmentDownloadServiceTest) TestDownloadOfAPendingUploadIsAConflict() {
	caller := AttachmentsTestSetup{}.Member(AttachmentsTestSetup{}.Domain(), "ana")
	upload, _ := AttachmentsTestSetup{}.Uploaded(caller, "hello", "text/plain")
	_, err := AttachmentsTestSetup{}.Downloader().download(caller, AccessRequest{AttachmentId: upload.AttachmentId})
	conflict := &AttachmentConflict{}
	Assert.IsTrue(errors.As(err, &conflict), "a pending upload got a link")
	Assert.Equals("state", conflict.Field)
}

func (test *AttachmentDownloadServiceTest) TestDownloadFailsWhenTheStoreHasNoSecret() {
	domainId := AttachmentsTestSetup{}.Domain()
	upload := test.ready(domainId)
	settings := AttachmentsTestSetup{}.Settings("")
	settings.Link.Secret = ""
	record, _ := AttachmentsTestSetup{}.Records().Lookup(domainId, upload.AttachmentId)
	_, err := AttachmentsTestSetup{}.Downloader().signed(settings, record)
	unavailable := &StorageUnavailable{}
	Assert.IsTrue(errors.As(err, &unavailable), "signed without a secret")
}
