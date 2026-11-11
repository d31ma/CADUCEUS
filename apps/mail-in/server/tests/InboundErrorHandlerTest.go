// @Test(InboundTestSetup, {"partition":"tests"})
type InboundErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *InboundErrorHandlerTest) TestInvalidAnswersInvalidRequestWithTheField() {
	answer := (&InboundErrorHandler{}).Invalid(&InvalidEnvelope{Field: "size", Reason: "too large"})
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("size", answer.Field)
}

func (test *InboundErrorHandlerTest) TestUnavailableAnswersUnavailable() {
	answer := (&InboundErrorHandler{}).Unavailable(&StorageUnavailable{Operation: "insert"})
	Assert.Equals("unavailable", answer.Code)
}

func (test *InboundErrorHandlerTest) TestInvalidEnvelopeNamesTheField() {
	Assert.Equals("size: too large", (&InvalidEnvelope{Field: "size", Reason: "too large"}).Error())
}

func (test *InboundErrorHandlerTest) TestStorageUnavailableNamesTheOperation() {
	Assert.Equals("inbound storage failed: insert", (&StorageUnavailable{Operation: "insert"}).Error())
}
