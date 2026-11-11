// @Test(HealthTestSetup, {"partition":"tests"})
type RealtimeErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *RealtimeErrorHandlerTest) TestInvalidNamesTheField() {
	answer := (&RealtimeErrorHandler{}).Invalid(&InvalidRequest{Field: "state", Reason: "unknown state"})
	Assert.Equals("state", answer.Field)
}

func (test *RealtimeErrorHandlerTest) TestUnauthenticatedAsksToSignInAgain() {
	Assert.Equals("unauthenticated", (&RealtimeErrorHandler{}).Unauthenticated(&Unauthenticated{}).Code)
}

func (test *RealtimeErrorHandlerTest) TestRefusedIsForbidden() {
	Assert.Equals("forbidden", (&RealtimeErrorHandler{}).Refused(&Forbidden{}).Code)
}

func (test *RealtimeErrorHandlerTest) TestMissingIsNotFound() {
	Assert.Equals("not_found", (&RealtimeErrorHandler{}).Missing(&NotFound{}).Code)
}

func (test *RealtimeErrorHandlerTest) TestUnavailableAsksToTryAgain() {
	Assert.Equals("unavailable", (&RealtimeErrorHandler{}).Unavailable(&Unavailable{}).Code)
}
