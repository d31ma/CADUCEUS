// @Test(AttachmentsTestSetup, {"partition":"tests"})
type AttachmentsErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *AttachmentsErrorHandlerTest) TestInvalidNamesTheField() {
	answer := (&AttachmentsErrorHandler{}).Invalid(&InvalidUpload{Field: "sha256", Reason: "bad"})
	Assert.Equals(ApiError{Code: "invalid_request", Message: "bad", Field: "sha256"}, answer)
}

func (test *AttachmentsErrorHandlerTest) TestRefusedAsksToSignIn() {
	Assert.Equals("unauthenticated", (&AttachmentsErrorHandler{}).Refused(&Unauthenticated{}).Code)
}

func (test *AttachmentsErrorHandlerTest) TestForbiddenKeepsTheMessage() {
	answer := (&AttachmentsErrorHandler{}).Forbidden(&Forbidden{Message: "no"})
	Assert.Equals(ApiError{Code: "forbidden", Message: "no"}, answer)
}

func (test *AttachmentsErrorHandlerTest) TestMissingIsNotFound() {
	Assert.Equals("not_found", (&AttachmentsErrorHandler{}).Missing(&AttachmentNotFound{}).Code)
}

func (test *AttachmentsErrorHandlerTest) TestConflictingNamesTheField() {
	answer := (&AttachmentsErrorHandler{}).Conflicting(&AttachmentConflict{Field: "size", Reason: "smaller"})
	Assert.Equals(ApiError{Code: "conflict", Message: "smaller", Field: "size"}, answer)
}

func (test *AttachmentsErrorHandlerTest) TestLargeNamesTheLimit() {
	answer := (&AttachmentsErrorHandler{}).Large(&AttachmentTooLarge{Limit: 9})
	Assert.Equals(ApiError{Code: "too_large", Message: "the file is larger than 9 bytes", Field: "size"}, answer)
}

func (test *AttachmentsErrorHandlerTest) TestUnavailableAsksToTryAgain() {
	Assert.Equals("unavailable", (&AttachmentsErrorHandler{}).Unavailable(&StorageUnavailable{Operation: "read"}).Code)
}

func (test *AttachmentsErrorHandlerTest) TestErrorsDescribeThemselves() {
	Assert.Equals("the attachment was not found", (&AttachmentNotFound{}).Error())
	Assert.Equals("size: smaller", (&AttachmentConflict{Field: "size", Reason: "smaller"}).Error())
	Assert.Equals("sha256: bad", (&InvalidUpload{Field: "sha256", Reason: "bad"}).Error())
	Assert.Equals("attachment storage failed during read", (&StorageUnavailable{Operation: "read"}).Error())
	Assert.Equals("the access token was refused", (&Unauthenticated{}).Error())
	Assert.Equals("no", (&Forbidden{Message: "no"}).Error())
}
