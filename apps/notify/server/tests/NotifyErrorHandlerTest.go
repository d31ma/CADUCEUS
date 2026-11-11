// @Test(NotifyTestSetup, {"partition":"tests"})
type NotifyErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *NotifyErrorHandlerTest) TestInvalidAnswersInvalidRequestWithTheField() {
	answer := (&NotifyErrorHandler{}).Invalid(&InvalidRequest{Field: "endpoint", Reason: "bad"})
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("endpoint", answer.Field)
}

func (test *NotifyErrorHandlerTest) TestRefusedAnswersUnauthenticated() {
	Assert.Equals("unauthenticated", (&NotifyErrorHandler{}).Refused(&Unauthenticated{}).Code)
}

func (test *NotifyErrorHandlerTest) TestForbiddenAnswersForbidden() {
	Assert.Equals("forbidden", (&NotifyErrorHandler{}).Forbidden(&NotService{}).Code)
}

func (test *NotifyErrorHandlerTest) TestMissingAnswersNotFound() {
	Assert.Equals("not_found", (&NotifyErrorHandler{}).Missing(&NotFound{What: "notification"}).Code)
}

func (test *NotifyErrorHandlerTest) TestUnavailableAnswersUnavailable() {
	Assert.Equals("unavailable", (&NotifyErrorHandler{}).Unavailable(&StorageUnavailable{Operation: "insert"}).Code)
}

func (test *NotifyErrorHandlerTest) TestPushUnavailableAnswersUnavailable() {
	Assert.Equals("unavailable", (&NotifyErrorHandler{}).PushUnavailable(&PushNotConfigured{}).Code)
}

func (test *NotifyErrorHandlerTest) TestErrorsDescribeThemselves() {
	Assert.Equals("endpoint: bad", (&InvalidRequest{Field: "endpoint", Reason: "bad"}).Error())
	Assert.Equals("the notification was not found", (&NotFound{What: "notification"}).Error())
	Assert.Equals("notify storage failed: insert", (&StorageUnavailable{Operation: "insert"}).Error())
	Assert.IsTrue((&NotService{}).Error() != "")
	Assert.IsTrue((&PushNotConfigured{}).Error() != "")
	Assert.IsTrue((&Unauthenticated{}).Error() != "")
}
