// @Test(HealthTestSetup, {"partition":"tests"})
type DeviceboxErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *DeviceboxErrorHandlerTest) TestEveryAnswerCarriesItsNumberAndCode() {
	handler := &DeviceboxErrorHandler{}
	Assert.Equals(ApiError{Code: "invalid_request", Message: "[MS6003]: bad id", Field: "envelopeId"}, handler.Invalid(&InvalidRequest{Message: "bad id", Field: "envelopeId", ErrorNumber: 6003}))
	Assert.Equals(ApiError{Code: "unauthenticated", Message: "[MS6001]: Sign in again."}, handler.Unauthenticated(&Unauthenticated{Message: "Sign in again.", ErrorNumber: 6001}))
	Assert.Equals(ApiError{Code: "forbidden", Message: "[MS6002]: no"}, handler.Forbidden(&Forbidden{Message: "no", ErrorNumber: 6002}))
	Assert.Equals(ApiError{Code: "not_found", Message: "[MS6014]: gone"}, handler.Missing(&NotFound{Message: "gone", ErrorNumber: 6014}))
	Assert.Equals(ApiError{Code: "duplicate", Message: "[MS6012]: twice"}, handler.Duplicate(&Duplicate{Message: "twice", ErrorNumber: 6012}))
	Assert.Equals(ApiError{Code: "too_large", Message: "[MS6009]: big"}, handler.TooLarge(&TooLarge{Message: "big", ErrorNumber: 6009}))
	Assert.Equals(ApiError{Code: "box_full", Message: "[MS6013]: full"}, handler.BoxFull(&BoxFull{Message: "full", ErrorNumber: 6013}))
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS6021]: down"}, handler.Unavailable(&Unavailable{Message: "down", ErrorNumber: 6021}))
	Assert.Equals(ApiError{Code: "unavailable", Message: "[MS6000]: Something went wrong."}, handler.Unexpected(nil))
}

func (test *DeviceboxErrorHandlerTest) TestUnknownFailuresFallBackToTheRouteNumber() {
	mapping := &ErrorMappingService{}
	Assert.IsTrue(mapping.Mailbox(nil) == nil, "nil should stay nil")
	Assert.Equals(Unavailable{}.MailboxRouteFailed(), mapping.Mailbox(Unavailable{}.Unexpected()).(*Unavailable))
	Assert.Equals(Unavailable{}.OutboxRouteFailed(), mapping.Outbox(Unavailable{}.Unexpected()).(*Unavailable))
}
