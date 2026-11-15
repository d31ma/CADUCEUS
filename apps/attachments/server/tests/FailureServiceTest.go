import (
	"errors"

	"mesail.dev/packages/attachments"
	"mesail.dev/packages/blobs"
)

// @Test(HealthTestSetup, {"partition":"tests"})
type FailureServiceTest struct {
	behaviors *BehaviorContext
}

func (test *FailureServiceTest) TestNoFailureStaysNil() {
	Assert.Equals(nil, (&FailureService{}).Translated(nil))
}

func (test *FailureServiceTest) TestAnInvalidUploadKeepsItsCaseNumber() {
	answer := (&FailureService{}).Translated(attachments.Invalid{}.MediaType())
	Assert.Equals(&InvalidUpload{Field: "mediaType", Reason: "Give a media type such as image/png.", ErrorNumber: 3502}, answer)
}

func (test *FailureServiceTest) TestAConflictKeepsItsCaseNumber() {
	answer := (&FailureService{}).Translated(attachments.Conflict{}.NotReady())
	Assert.Equals(&AttachmentConflict{Field: "state", Reason: "The attachment is not ready to download.", ErrorNumber: 3511}, answer)
}

func (test *FailureServiceTest) TestOtherFailuresKeepTheirTypes() {
	failures := &FailureService{}
	Assert.Equals(&AttachmentTooLarge{Limit: 9, ErrorNumber: 24}, failures.Translated(blobs.TooLarge{Limit: 9}))
	Assert.Equals(&AttachmentNotFound{}, failures.Translated(attachments.Missing{}))
	Assert.Equals(&Forbidden{Message: "Only a service may read attachment details."}, failures.Translated(attachments.Forbidden{}))
	Assert.Equals(&Unauthenticated{}, failures.Translated(attachments.Unauthenticated{}))
	Assert.Equals(&StorageUnavailable{Operation: "boom"}, failures.Translated(errors.New("boom")))
}
