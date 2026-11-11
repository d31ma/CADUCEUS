import "errors"

// @Test(SessionTestSetup, {"partition":"tests"})
type SessionProblemServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionProblemServiceTest) TestOfStateAcceptsTheSameState() {
	Assert.Equals("", (&SessionProblemService{}).OfState("state-1", "state-1"))
}

func (test *SessionProblemServiceTest) TestOfStateRefusesAnotherState() {
	Assert.Equals("startAgain", (&SessionProblemService{}).OfState("state-1", "state-2"))
}

func (test *SessionProblemServiceTest) TestOfStateRefusesWhenNothingWasStarted() {
	Assert.Equals("startAgain", (&SessionProblemService{}).OfState("", ""))
}

func (test *SessionProblemServiceTest) TestOfFailureIsEmptyWithoutFailure() {
	Assert.Equals("", (&SessionProblemService{}).OfFailure(nil))
}

func (test *SessionProblemServiceTest) TestOfFailureNamesAForbiddenAccount() {
	Assert.Equals("accountOff", (&SessionProblemService{}).OfFailure(&BehaviorError{Code: "HTTP_STATUS", Status: 403}))
}

func (test *SessionProblemServiceTest) TestOfFailureCallsOtherStatusesUnreachable() {
	Assert.Equals("idpDown", (&SessionProblemService{}).OfFailure(&BehaviorError{Code: "HTTP_STATUS", Status: 502}))
}

func (test *SessionProblemServiceTest) TestOfFailureCallsPlainErrorsUnreachable() {
	Assert.Equals("idpDown", (&SessionProblemService{}).OfFailure(errors.New("offline")))
}

func (test *SessionProblemServiceTest) TestOfSignInErrorNamesACancel() {
	Assert.Equals("cancelled", (&SessionProblemService{}).OfSignInError("access_denied"))
}

func (test *SessionProblemServiceTest) TestOfSignInErrorCallsUnknownCodesUnreachable() {
	Assert.Equals("idpDown", (&SessionProblemService{}).OfSignInError("server_error"))
}

func (test *SessionProblemServiceTest) TestUnavailableNamesTheProblem() {
	Assert.Equals("unavailable", (&SessionProblemService{}).Unavailable())
}
