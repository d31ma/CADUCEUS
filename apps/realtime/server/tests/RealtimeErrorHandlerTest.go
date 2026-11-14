import "errors"

// @Test(HealthTestSetup, {"partition":"tests"})
type RealtimeErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *RealtimeErrorHandlerTest) TestInvalidNamesTheFieldAndTheNumber() {
	answer := (&RealtimeErrorHandler{}).Invalid(&InvalidRequest{Message: "unknown state", Field: "state", ErrorNumber: 5007})
	Assert.Equals("state", answer.Field)
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("[MS5007]: unknown state", answer.Message)
}

func (test *RealtimeErrorHandlerTest) TestUnauthenticatedAsksToSignInAgain() {
	answer := (&RealtimeErrorHandler{}).Unauthenticated(&Unauthenticated{ErrorNumber: 5011})
	Assert.Equals("unauthenticated", answer.Code)
	Assert.Equals("[MS5011]: Sign in again.", answer.Message)
}

func (test *RealtimeErrorHandlerTest) TestRefusedIsForbidden() {
	answer := (&RealtimeErrorHandler{}).Refused(Forbidden{}.EventCallerNotAService())
	Assert.Equals("forbidden", answer.Code)
	Assert.Equals("[MS5018]: This caller may not use this route.", answer.Message)
}

func (test *RealtimeErrorHandlerTest) TestMissingIsNotFound() {
	answer := (&RealtimeErrorHandler{}).Missing(&NotFound{ErrorNumber: 5019})
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("[MS5019]: Nothing was found.", answer.Message)
}

func (test *RealtimeErrorHandlerTest) TestUnavailableAsksToTryAgain() {
	answer := (&RealtimeErrorHandler{}).Unavailable(Unavailable{}.RouteFailed())
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("[MS5024]: Realtime is not available right now. Try again later.", answer.Message)
}

func (test *RealtimeErrorHandlerTest) TestAnUnexpectedErrorHasTheFirstNumber() {
	answer := (&RealtimeErrorHandler{}).Unexpected(errors.New("boom"))
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("[MS5000]: Something went wrong.", answer.Message)
}
