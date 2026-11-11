import "errors"

// @Test(SessionTestSetup, {"partition":"tests"})
type SessionProblemServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionProblemServiceTest) TestOfFailureIsEmptyWithoutFailure() {
	Assert.Equals("", (&SessionProblemService{}).OfFailure(nil))
}

func (test *SessionProblemServiceTest) TestOfFailureNamesWrongCredentials() {
	Assert.Equals("wrongCredentials", (&SessionProblemService{}).OfFailure(&BehaviorError{Code: "HTTP_STATUS", Status: 401}))
}

func (test *SessionProblemServiceTest) TestOfFailureNamesTooManyTries() {
	Assert.Equals("rateLimited", (&SessionProblemService{}).OfFailure(&BehaviorError{Code: "HTTP_STATUS", Status: 429}))
}

func (test *SessionProblemServiceTest) TestOfFailureCallsOtherStatusesUnavailable() {
	Assert.Equals("unavailable", (&SessionProblemService{}).OfFailure(&BehaviorError{Code: "HTTP_STATUS", Status: 502}))
}

func (test *SessionProblemServiceTest) TestOfFailureCallsPlainErrorsUnavailable() {
	Assert.Equals("unavailable", (&SessionProblemService{}).OfFailure(errors.New("offline")))
}
