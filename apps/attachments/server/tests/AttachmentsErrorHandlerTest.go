import (
	"errors"

	"mesail.dev/packages/apierrors"
	"mesail.dev/packages/attachments"
)

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type AttachmentsErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *AttachmentsErrorHandlerTest) TestInvalidNamesTheField() {
	answer := (&AttachmentsErrorHandler{}).Invalid(&InvalidUpload{Field: "sha256", Reason: "bad", ErrorNumber: 3504})
	Assert.Equals(ApiError{Code: "invalid_request", Message: "[MS3504]: bad", Field: "sha256"}, answer)
}

func (test *AttachmentsErrorHandlerTest) TestRefusedAsksToSignIn() {
	answer := (&AttachmentsErrorHandler{}).Refused(&Unauthenticated{})
	Assert.Equals(ApiError{Code: "unauthenticated", Message: "[MS3505]: Sign in again."}, answer)
}

func (test *AttachmentsErrorHandlerTest) TestForbiddenKeepsTheMessage() {
	answer := (&AttachmentsErrorHandler{}).Forbidden(&Forbidden{Message: "no"})
	Assert.Equals(ApiError{Code: "forbidden", Message: "[MS3506]: no"}, answer)
}

func (test *AttachmentsErrorHandlerTest) TestMissingIsNotFound() {
	answer := (&AttachmentsErrorHandler{}).Missing(&AttachmentNotFound{})
	Assert.Equals(ApiError{Code: "not_found", Message: "[MS3507]: The attachment was not found."}, answer)
}

func (test *AttachmentsErrorHandlerTest) TestConflictingNamesTheField() {
	answer := (&AttachmentsErrorHandler{}).Conflicting(&AttachmentConflict{Field: "size", Reason: "smaller", ErrorNumber: 3509})
	Assert.Equals(ApiError{Code: "conflict", Message: "[MS3509]: smaller", Field: "size"}, answer)
}

func (test *AttachmentsErrorHandlerTest) TestLargeNamesTheLimit() {
	answer := (&AttachmentsErrorHandler{}).Large(&AttachmentTooLarge{Limit: 9})
	Assert.Equals(ApiError{Code: "too_large", Message: "[MS3512]: the file is larger than 9 bytes", Field: "size"}, answer)
}

func (test *AttachmentsErrorHandlerTest) TestUnavailableAsksToTryAgain() {
	answer := (&AttachmentsErrorHandler{}).Unavailable(&StorageUnavailable{Operation: "read"})
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS3513]: Attachment storage is not available. Try again later."}, answer)
}

func (test *AttachmentsErrorHandlerTest) TestAnyOtherErrorIsUnexpected() {
	answer := (&AttachmentsErrorHandler{}).Unexpected(errors.New("disk on fire"))
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS3500]: Something went wrong."}, answer)
}

func (test *AttachmentsErrorHandlerTest) TestEveryNumberIsUsedOnceInsideTheAttachmentsBlock() {
	numbers := []int{
		(&UnexpectedFailure{}).Number(),
		attachments.Invalid{}.FileName().ErrorNumber, attachments.Invalid{}.MediaType().ErrorNumber,
		attachments.Invalid{}.Size().ErrorNumber, attachments.Invalid{}.Sha256().ErrorNumber,
		(&Unauthenticated{}).Number(), (&Forbidden{}).Number(), (&AttachmentNotFound{}).Number(),
		attachments.Conflict{}.NotUploaded().ErrorNumber, attachments.Conflict{}.SizeMismatch().ErrorNumber,
		attachments.Conflict{}.Sha256Mismatch().ErrorNumber, attachments.Conflict{}.NotReady().ErrorNumber,
		(&AttachmentTooLarge{}).Number(), (&StorageUnavailable{}).Number(),
	}
	block := apierrors.NumberBlock{First: 3500, Last: 3999}
	Assert.Equals([]int{}, AttachmentsTestSetup{}.OutsideBlock(block, numbers))
	Assert.Equals(len(numbers), AttachmentsTestSetup{}.DistinctCount(numbers))
}

func (test *AttachmentsErrorHandlerTest) TestErrorsDescribeThemselves() {
	Assert.Equals("the attachment was not found", (&AttachmentNotFound{}).Error())
	Assert.Equals("size: smaller", (&AttachmentConflict{Field: "size", Reason: "smaller"}).Error())
	Assert.Equals("sha256: bad", (&InvalidUpload{Field: "sha256", Reason: "bad"}).Error())
	Assert.Equals("attachment storage failed during read", (&StorageUnavailable{Operation: "read"}).Error())
	Assert.Equals("the access token was refused", (&Unauthenticated{}).Error())
	Assert.Equals("no", (&Forbidden{Message: "no"}).Error())
	Assert.Equals("an unexpected error", (&UnexpectedFailure{}).Error())
}
