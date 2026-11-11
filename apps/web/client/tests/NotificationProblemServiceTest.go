// @Test(NotificationTestSetup, {"partition":"tests"})
type NotificationProblemServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationProblemServiceTest) TestOfFailureNamesTheProblem() {
	problems := &NotificationProblemService{}
	Assert.Equals("", problems.OfFailure(nil))
	Assert.Equals("session", problems.OfFailure(&BehaviorError{Status: 401}))
	Assert.Equals("error", problems.OfFailure(&BehaviorError{Status: 503}))
	Assert.Equals("waiting", problems.OfFailure(problems.SignedOut()))
}
