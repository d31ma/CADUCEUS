import (
	"errors"
)

// @Test(InboundTestSetup, {"partition":"tests"})
type InboundErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *InboundErrorHandlerTest) TestInvalidAnswersInvalidRequestWithTheNumberAndTheField() {
	answer := (&InboundErrorHandler{}).Invalid(&InvalidEnvelope{Field: "size", Reason: "the size must be between 1 byte and 64 MiB", ErrorNumber: 2506})
	Assert.Equals(ApiError{Code: "invalid_request", Message: "[MS2506]: the size must be between 1 byte and 64 MiB", Field: "size"}, answer)
}

func (test *InboundErrorHandlerTest) TestRefusedAnswersUnauthenticatedWithTheNumber() {
	answer := (&InboundErrorHandler{}).Refused(&Unauthenticated{})
	Assert.Equals(ApiError{Code: "unauthenticated", Message: "[MS2509]: The access token was refused."}, answer)
}

func (test *InboundErrorHandlerTest) TestMissingAnswersNotFoundWithTheNumber() {
	answer := (&InboundErrorHandler{}).Missing(&NotFound{Message: "no such outbox document", ErrorNumber: 2513})
	Assert.Equals(ApiError{Code: "not_found", Message: "[MS2513]: no such outbox document"}, answer)
}

func (test *InboundErrorHandlerTest) TestUnavailableAnswersUnavailableWithTheNumber() {
	answer := (&InboundErrorHandler{}).Unavailable(&StorageUnavailable{Operation: "insert /secret/path"})
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS2514]: Inbound storage is not available. Try again later."}, answer)
}

func (test *InboundErrorHandlerTest) TestUnexpectedAnswersTheUnexpectedErrorNumber() {
	answer := (&InboundErrorHandler{}).Unexpected(errors.New("a failure no other method takes"))
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS2500]: Something went wrong."}, answer)
}

func (test *InboundErrorHandlerTest) TestInvalidEnvelopeNamesTheField() {
	Assert.Equals("size: too large", (&InvalidEnvelope{Field: "size", Reason: "too large"}).Error())
}

func (test *InboundErrorHandlerTest) TestStorageUnavailableNamesTheOperation() {
	Assert.Equals("inbound storage failed: insert", (&StorageUnavailable{Operation: "insert"}).Error())
}

func (test *InboundErrorHandlerTest) TestUnauthenticatedSaysTheTokenWasRefused() {
	Assert.Equals("the access token was refused", (&Unauthenticated{}).Error())
}
