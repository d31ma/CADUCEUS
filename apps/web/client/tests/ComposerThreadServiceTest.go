// @Test(ComposerTestSetup, {"partition":"tests"})
type ComposerThreadServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ComposerThreadServiceTest) thread() *ComposerThreadService {
	thread := ComposerTestSetup{}.Thread(test.behaviors)
	thread.Open(SendThreadTarget{ConversationId: "conv-1", ThreadRootId: "msg-root", Title: "#support", ExternalAddress: "carol@partner.org"})
	return thread
}

func (test *ComposerThreadServiceTest) TestOpenShowsAnInternalReplyByDefault() {
	thread := test.thread()
	Assert.IsTrue(!thread.SendToExternal)
	Assert.Equals("Send reply", thread.SendLabel)
	Assert.Equals("example.com", thread.DomainId)
}

func (test *ComposerThreadServiceTest) TestChooseExternalAlsoSendsTheEmail() {
	thread := test.thread()
	thread.ChooseExternal(true)
	Assert.Equals("Send reply and email", thread.SendLabel)
	Assert.IsTrue(thread.request().Email.SendToExternal)
}

func (test *ComposerThreadServiceTest) TestAnInternalReplyHasNoEmailOptions() {
	thread := test.thread()
	thread.Body = "hello"
	request := thread.request()
	Assert.IsTrue(request.Email == nil)
	Assert.Equals("msg-root", request.ThreadRootId)
	Assert.Equals("conv-1", request.ConversationId)
}

func (test *ComposerThreadServiceTest) TestSendWithAnEmptyReplyAsksForText() {
	thread := test.thread()
	thread.SendWith("  ")
	Assert.Equals("emptyMessage", thread.Problem)
	Assert.Equals("Write a message or attach a file to send.", thread.ProblemText)
}

func (test *ComposerThreadServiceTest) TestSendWithSendsAndClears() {
	thread := test.thread()
	thread.ChooseExternal(true)
	thread.SendWith("Thanks, Carol.")
	Assert.Equals("", thread.Problem)
	Assert.Equals("", thread.Body)
	Assert.IsTrue(!thread.SendToExternal)
	Assert.Equals("Reply sent.", thread.Announcement)
}

func (test *ComposerThreadServiceTest) TestAFailedReplyKeepsTheTextAndItsId() {
	thread := test.thread()
	thread.SendWith("offline")
	Assert.Equals("unavailable", thread.Problem)
	Assert.Equals("offline", thread.Body)
	Assert.Equals("id-1", thread.send.PendingIds["thread"])
}

func (test *ComposerThreadServiceTest) TestOpenAnotherThreadStartsFresh() {
	thread := test.thread()
	thread.SendWith("offline")
	thread.Open(SendThreadTarget{ConversationId: "conv-2", ThreadRootId: "msg-2"})
	Assert.Equals("", thread.Body)
	Assert.Equals("", thread.send.PendingIds["thread"])
	Assert.Equals("", thread.Problem)
}
