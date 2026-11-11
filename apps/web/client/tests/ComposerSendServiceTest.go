// @Test(ComposerTestSetup, {"partition":"tests"})
type ComposerSendServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ComposerSendServiceTest) sender() *ComposerSendService {
	BrowserFakes{}.Answer("secureRandom", "uuid", "id-1")
	return ComposerTestSetup{}.Sender(test.behaviors)
}

func (test *ComposerSendServiceTest) TestSendGivesTheMessageAClientId() {
	result, failure := test.sender().Send(SendRequest{Body: "hello"})
	Assert.Equals(nil, failure)
	Assert.Equals("msg-id-1", result.Message.MessageId)
}

func (test *ComposerSendServiceTest) TestSendForgetsTheIdAfterSuccess() {
	sender := test.sender()
	sender.Send(SendRequest{Body: "hello"})
	Assert.Equals("", sender.PendingId)
}

func (test *ComposerSendServiceTest) TestSendKeepsTheIdForARetryAfterAFailure() {
	sender := test.sender()
	_, failure := sender.Send(SendRequest{Body: "offline"})
	Assert.IsTrue(failure != nil)
	Assert.Equals("id-1", sender.PendingId)
	BrowserFakes{}.Answer("secureRandom", "uuid", "id-2")
	result, _ := sender.Send(SendRequest{Body: "hello"})
	Assert.Equals("msg-id-1", result.Message.MessageId)
}

func (test *ComposerSendServiceTest) TestResetStartsANewMessage() {
	sender := test.sender()
	sender.Send(SendRequest{Body: "offline"})
	sender.Reset()
	Assert.Equals("", sender.PendingId)
}

func (test *ComposerSendServiceTest) TestASignedOutAnswerRenewsTheTokenAndSendsAgain() {
	sender := test.sender()
	result, failure := sender.againWhenSignedOut(SendRequest{ClientMessageId: "id-9", Body: "hello"}, SendResult{}, &BehaviorError{Code: "HTTP_STATUS", Status: 401})
	Assert.Equals(nil, failure)
	Assert.Equals("msg-id-9", result.Message.MessageId)
}

func (test *ComposerSendServiceTest) TestOtherFailuresAreNotSentAgain() {
	sender := test.sender()
	_, failure := sender.againWhenSignedOut(SendRequest{Body: "hello"}, SendResult{}, &BehaviorError{Code: "HTTP_STATUS", Status: 503})
	Assert.Equals(503, BehaviorErrorStatus(failure))
}
