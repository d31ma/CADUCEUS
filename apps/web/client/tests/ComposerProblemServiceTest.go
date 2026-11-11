import "errors"

// @Test(ComposerTestSetup, {"partition":"tests"})
type ComposerProblemServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ComposerProblemServiceTest) problemOf(status int) string {
	return (&ComposerProblemService{}).OfFailure(&BehaviorError{Code: "HTTP_STATUS", Status: status})
}

func (test *ComposerProblemServiceTest) TestOfFailureIsEmptyWithoutFailure() {
	Assert.Equals("", (&ComposerProblemService{}).OfFailure(nil))
}

func (test *ComposerProblemServiceTest) TestOfFailureNamesRecipientsChanged() {
	Assert.Equals("recipientsChanged", test.problemOf(409))
}

func (test *ComposerProblemServiceTest) TestOfFailureNamesChatNotAllowed() {
	Assert.Equals("chatNotAllowed", test.problemOf(422))
}

func (test *ComposerProblemServiceTest) TestOfFailureNamesAnEndedSession() {
	Assert.Equals("signedOut", test.problemOf(401))
}

func (test *ComposerProblemServiceTest) TestOfFailureCallsServerErrorsUnavailable() {
	Assert.Equals("unavailable", test.problemOf(503))
}

func (test *ComposerProblemServiceTest) TestOfFailureCallsNetworkErrorsUnavailable() {
	Assert.Equals("unavailable", (&ComposerProblemService{}).OfFailure(errors.New("offline")))
}

func (test *ComposerProblemServiceTest) TestTextSaysNothingWasSentAfterAChange() {
	Assert.Equals("Not sent. Some recipients changed. Check how each one gets it, then send again.", (&ComposerProblemService{}).Text("recipientsChanged"))
}

func (test *ComposerProblemServiceTest) TestTextIsEmptyWithoutAProblem() {
	Assert.Equals("", (&ComposerProblemService{}).Text(""))
}
