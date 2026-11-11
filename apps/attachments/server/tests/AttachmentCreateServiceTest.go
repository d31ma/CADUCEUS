import (
	"errors"
	"strings"
)

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type AttachmentCreateServiceTest struct {
	behaviors *BehaviorContext
}

func (test *AttachmentCreateServiceTest) TestCreateRefusesACallerWithoutAToken() {
	_, err := AttachmentsTestSetup{}.Creator().Create("", AttachmentsTestSetup{}.Request("hello", "text/plain"))
	refused := &Unauthenticated{}
	Assert.IsTrue(errors.As(err, &refused), "no token was accepted")
}

func (test *AttachmentCreateServiceTest) TestCreateStoresAPendingRecordAndSignsAnUploadLink() {
	AttachmentsTestSetup{}.Settings("")
	caller := AttachmentsTestSetup{}.Member(AttachmentsTestSetup{}.Domain(), "ana")
	upload, err := AttachmentsTestSetup{}.Creator().create(caller, AttachmentsTestSetup{}.Request("hello", "text/plain"))
	record, _ := AttachmentsTestSetup{}.Records().Lookup(caller.DomainId, upload.AttachmentId)
	Assert.IsTrue(err == nil, "create failed")
	Assert.IsTrue(strings.HasPrefix(upload.AttachmentId, "att_"), upload.AttachmentId)
	Assert.IsTrue(strings.HasPrefix(upload.UploadUrl, "http://127.0.0.1:8100/blobs/"+caller.DomainId+"/"), upload.UploadUrl)
	Assert.IsTrue(upload.ExpiresAt != "", "no expiry")
	Assert.Equals("pending", record.State)
	Assert.Equals("ana", record.UploaderId)
	Assert.IsTrue(strings.HasPrefix(record.BlobKey, caller.DomainId+"/"), record.BlobKey)
}

func (test *AttachmentCreateServiceTest) TestCreateRefusesAnInvalidRequest() {
	AttachmentsTestSetup{}.Settings("")
	request := AttachmentsTestSetup{}.Request("hello", "text/plain")
	request.Sha256 = "nope"
	_, err := AttachmentsTestSetup{}.Creator().create(AttachmentsTestSetup{}.Member("example.com", "ana"), request)
	invalid := &InvalidUpload{}
	Assert.IsTrue(errors.As(err, &invalid), "invalid request accepted")
}

func (test *AttachmentCreateServiceTest) TestCreateRefusesWhenTheStoreCannotSign() {
	AttachmentsTestSetup{}.Settings("")
	caller := AttachmentsTestSetup{}.Member("Not A Domain", "ana")
	_, err := AttachmentsTestSetup{}.Creator().create(caller, AttachmentsTestSetup{}.Request("hello", "text/plain"))
	Assert.IsTrue(err != nil, "a bad domain was accepted")
}
