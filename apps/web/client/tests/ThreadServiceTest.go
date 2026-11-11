// @Test(ThreadTestSetup, {"partition":"tests"})
type ThreadServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ThreadServiceTest) remote(expiresAtMs float64) *ThreadRemoteService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	BrowserFakes{}.Answer("secureRandom", "uuid", "uuid-1")
	tokens := &SessionTokenService{
		identity: NewIdentityClientWith(test.behaviors),
		keys:     map[string]string{"example.com": "key-1"},
		tokens:   map[string]SessionTokens{"example.com": {AccessToken: "access-1", ExpiresAtMs: expiresAtMs}},
	}
	accounts := &SessionAccountService{}
	accounts.Add(Me{MemberId: "m-1", DomainId: "example.com", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com", Roles: []string{}})
	return &ThreadRemoteService{client: NewThreadClientWith(test.behaviors), tokens: tokens, accounts: accounts}
}

func (test *ThreadServiceTest) threadWith(remote *ThreadRemoteService) *ThreadService {
	shell := &ShellService{}
	shell.Start()
	return &ThreadService{shell: shell, announce: &ShellAnnounceService{}, remote: remote, rows: &ThreadRowService{}}
}

func (test *ThreadServiceTest) thread() *ThreadService {
	return test.threadWith(test.remote(3600000))
}

func (test *ThreadServiceTest) opened(rootId string) *ThreadService {
	thread := test.thread()
	thread.OpenMessage(ThreadOpenRequested{ConversationId: "conv-1", MessageId: rootId})
	thread.Load(ThreadLoadRequested{ConversationId: "conv-1", RootId: rootId})
	return thread
}

func (test *ThreadServiceTest) replying(text string) *ThreadService {
	thread := test.opened("root-1")
	thread.Write(text)
	thread.Reply()
	return thread
}

func (test *ThreadServiceTest) sent(text string) *ThreadService {
	thread := test.replying(text)
	thread.Send(ThreadReplyRequested{Request: thread.replyRequest()})
	return thread
}

func (test *ThreadServiceTest) replyIds(thread *ThreadService) []string {
	ids := []string{}
	for _, row := range thread.Replies {
		ids = append(ids, row.MessageId)
	}
	return ids
}

func (test *ThreadServiceTest) TestOpenMessageOpensTheThreadColumn() {
	thread := test.thread()
	thread.OpenMessage(ThreadOpenRequested{ConversationId: "conv-1", MessageId: "root-1"})
	Assert.IsTrue(thread.shell.ThreadOpen)
	Assert.Equals("thread", thread.shell.Pane)
	Assert.Equals("loading", thread.Status)
	Assert.Equals("root-1", thread.RootId)
}

func (test *ThreadServiceTest) TestLoadShowsRepliesOldestFirst() {
	thread := test.opened("root-1")
	Assert.Equals("ready", thread.Status)
	Assert.Equals([]string{"reply-2", "reply-3"}, test.replyIds(thread))
	Assert.Equals("c-2", thread.NextCursor)
}

func (test *ThreadServiceTest) TestLoadShowsTheRootFromItsOwnCall() {
	thread := test.opened("root-1")
	Assert.IsTrue(thread.HasRoot)
	Assert.Equals("Ana Ruiz", thread.Root.AuthorName)
	Assert.Equals("09:12", thread.Root.Time)
	Assert.Equals(2, len(thread.Replies))
}

func (test *ThreadServiceTest) TestLoadCountsRepliesFromTheRoot() {
	Assert.Equals("3 replies", test.opened("root-1").RepliesLabel)
}

func (test *ThreadServiceTest) TestShowOlderAddsOlderRepliesFirst() {
	thread := test.opened("root-1")
	thread.ShowOlder()
	Assert.IsTrue(thread.LoadingOlder)
	thread.LoadOlder(ThreadOlderRequested{ConversationId: "conv-1", RootId: "root-1", Cursor: "c-2"})
	Assert.Equals([]string{"reply-1", "reply-2", "reply-3"}, test.replyIds(thread))
	Assert.Equals("", thread.NextCursor)
	Assert.IsTrue(!thread.LoadingOlder)
}

func (test *ThreadServiceTest) TestLoadFailsWhenTheRootDoesNotLoad() {
	thread := test.opened("root-2")
	Assert.Equals("error", thread.Status)
}

func (test *ThreadServiceTest) TestLoadOfARemovedRootSaysItIsGone() {
	thread := test.thread()
	thread.OpenMessage(ThreadOpenRequested{ConversationId: "conv-1", MessageId: "root-1"})
	thread.applyWhenCurrent(ThreadLoadRequested{ConversationId: "conv-1", RootId: "root-1"}, ThreadMessage{}, ThreadMessagePage{}, &BehaviorError{Status: 404})
	Assert.Equals("gone", thread.Status)
}

func (test *ThreadServiceTest) TestLoadOlderIgnoresAnAnswerForAnotherThread() {
	thread := test.opened("root-1")
	thread.LoadOlder(ThreadOlderRequested{ConversationId: "conv-1", RootId: "root-9", Cursor: "c-2"})
	Assert.Equals(2, len(thread.Replies))
}

func (test *ThreadServiceTest) TestOpenShowsTheGivenRootAtOnce() {
	thread := test.thread()
	root := ThreadMessage{MessageId: "root-2", ConversationId: "conv-1", Author: ThreadAuthor{DisplayName: "Ana Ruiz"}, Body: "Hello"}
	thread.Open(ThreadRootOpenRequested{Root: root})
	Assert.IsTrue(thread.HasRoot)
	Assert.Equals("root-2", thread.RootId)
	Assert.Equals("Hello", thread.Root.Body)
	Assert.IsTrue(thread.shell.ThreadOpen)
}

func (test *ThreadServiceTest) TestLoadReplacesTheGivenRootWithTheServerCopy() {
	thread := test.thread()
	thread.Open(ThreadRootOpenRequested{Root: ThreadMessage{MessageId: "root-1", ConversationId: "conv-1", Body: "Old text"}})
	thread.Load(ThreadLoadRequested{ConversationId: "conv-1", RootId: "root-1"})
	Assert.IsTrue(thread.HasRoot)
	Assert.Equals("Build 42 is green.", thread.Root.Body)
}

func (test *ThreadServiceTest) TestOpenOnAReplyOpensItsRoot() {
	thread := test.thread()
	thread.Open(ThreadRootOpenRequested{Root: ThreadMessage{MessageId: "reply-2", ConversationId: "conv-1", ThreadRootId: "root-1"}})
	Assert.Equals("root-1", thread.RootId)
	Assert.IsTrue(!thread.HasRoot)
}

func (test *ThreadServiceTest) TestLoadFailureShowsTheErrorState() {
	thread := test.opened("broken")
	Assert.Equals("error", thread.Status)
	Assert.Equals(0, len(thread.Replies))
}

func (test *ThreadServiceTest) TestLoadAfterTheSessionEndedShowsTheSessionState() {
	thread := test.threadWith(test.remote(0))
	thread.remote.tokens.keys["example.com"] = "ended"
	thread.OpenMessage(ThreadOpenRequested{ConversationId: "conv-1", MessageId: "root-1"})
	thread.Load(ThreadLoadRequested{ConversationId: "conv-1", RootId: "root-1"})
	Assert.Equals("session", thread.Status)
}

func (test *ThreadServiceTest) TestLoadIgnoresAnAnswerForAnotherThread() {
	thread := test.thread()
	thread.OpenMessage(ThreadOpenRequested{ConversationId: "conv-1", MessageId: "root-1"})
	thread.OpenMessage(ThreadOpenRequested{ConversationId: "conv-1", MessageId: "root-2"})
	thread.Load(ThreadLoadRequested{ConversationId: "conv-1", RootId: "root-1"})
	Assert.Equals("loading", thread.Status)
	Assert.Equals(0, len(thread.Replies))
}

func (test *ThreadServiceTest) TestReloadShowsLoadingAgain() {
	thread := test.opened("broken")
	thread.Reload()
	Assert.Equals("loading", thread.Status)
}

func (test *ThreadServiceTest) TestCloseClosesTheColumnAndForgetsTheThread() {
	thread := test.opened("root-1")
	thread.Close()
	Assert.IsTrue(!thread.shell.ThreadOpen)
	Assert.Equals("main", thread.shell.Pane)
	Assert.Equals("", thread.RootId)
	Assert.Equals("", thread.Status)
	Assert.Equals(0, len(thread.Replies))
}

func (test *ThreadServiceTest) TestRefreshKeepsTheRepliesWhileLoading() {
	thread := test.opened("root-1")
	thread.Refresh(ThreadRefreshRequested{ConversationId: "conv-1"})
	Assert.Equals("ready", thread.Status)
	Assert.Equals(2, len(thread.Replies))
}

func (test *ThreadServiceTest) TestRefreshIgnoresAnotherConversation() {
	thread := test.opened("root-1")
	thread.Refresh(ThreadRefreshRequested{ConversationId: "conv-2"})
	Assert.Equals("conv-1", thread.ConversationId)
}

func (test *ThreadServiceTest) TestRefreshDoesNothingWhenClosed() {
	thread := test.thread()
	thread.Refresh(ThreadRefreshRequested{ConversationId: "conv-1"})
	Assert.Equals("", thread.Status)
}

func (test *ThreadServiceTest) TestShownKeepsTheOpenThread() {
	thread := test.opened("root-1")
	thread.Shown(VisibilityChanged{Visible: true})
	Assert.Equals("ready", thread.Status)
}

func (test *ThreadServiceTest) TestShownIgnoresAHiddenPage() {
	thread := test.thread()
	thread.Shown(VisibilityChanged{Visible: false})
	Assert.Equals("", thread.RootId)
}

func (test *ThreadServiceTest) TestWriteAllowsSendOnlyWithText() {
	thread := test.opened("root-1")
	thread.Write("   ")
	Assert.IsTrue(!thread.CanSend)
	thread.Write("Thanks")
	Assert.IsTrue(thread.CanSend)
}

func (test *ThreadServiceTest) TestReplyWithAnEmptyDraftSendsNothing() {
	thread := test.opened("root-1")
	thread.Reply()
	Assert.Equals("", thread.SendStatus)
}

func (test *ThreadServiceTest) TestReplyShowsTheReplyAsSending() {
	thread := test.replying("  Thanks  ")
	Assert.Equals("sending", thread.SendStatus)
	Assert.Equals("Thanks", thread.Pending.Body)
	Assert.Equals("Ana Ruiz", thread.Pending.AuthorName)
	Assert.IsTrue(!thread.CanSend)
}

func (test *ThreadServiceTest) TestReplyAsksForAThreadReply() {
	request := test.replying("Thanks").replyRequest()
	Assert.Equals("uuid-1", request.ClientMessageId)
	Assert.Equals("conv-1", request.ConversationId)
	Assert.Equals("root-1", request.ThreadRootId)
	Assert.Equals("Thanks", request.Body)
	Assert.Equals(0, len(request.AttachmentIds))
}

func (test *ThreadServiceTest) TestReplyAlsoSendsToTheConversationWhenTicked() {
	thread := test.opened("root-1")
	thread.AlsoToConversation = true
	thread.Write("Thanks")
	thread.Reply()
	Assert.IsTrue(thread.replyRequest().AlsoToConversation)
}

func (test *ThreadServiceTest) TestReplyStaysInTheThreadByDefault() {
	Assert.IsTrue(!test.replying("Thanks").replyRequest().AlsoToConversation)
}

func (test *ThreadServiceTest) TestSendResetsAlsoToConversation() {
	thread := test.opened("root-1")
	thread.AlsoToConversation = true
	thread.Write("Thanks")
	thread.Reply()
	thread.Send(ThreadReplyRequested{Request: thread.replyRequest()})
	Assert.IsTrue(!thread.AlsoToConversation)
}

func (test *ThreadServiceTest) TestSendAddsTheReplyAndClearsTheDraft() {
	thread := test.sent("Thanks")
	Assert.Equals([]string{"reply-2", "reply-3", "msg-uuid-1"}, test.replyIds(thread))
	Assert.Equals("", thread.Draft)
	Assert.Equals("", thread.SendStatus)
	Assert.Equals("Reply sent", thread.announce.Message)
}

func (test *ThreadServiceTest) TestSendFailureKeepsTheDraft() {
	thread := test.sent("offline")
	Assert.Equals("failed", thread.SendStatus)
	Assert.Equals("offline", thread.Draft)
	Assert.Equals("Reply not sent", thread.announce.Message)
	Assert.IsTrue(thread.CanSend)
}

func (test *ThreadServiceTest) TestRetryKeepsTheSameClientMessageId() {
	thread := test.sent("offline")
	BrowserFakes{}.Answer("secureRandom", "uuid", "uuid-2")
	thread.Retry()
	Assert.Equals("sending", thread.SendStatus)
	Assert.Equals("uuid-1", thread.replyRequest().ClientMessageId)
}

func (test *ThreadServiceTest) TestSendAgainAfterFailureKeepsTheSameClientMessageId() {
	thread := test.sent("offline")
	BrowserFakes{}.Answer("secureRandom", "uuid", "uuid-2")
	thread.Reply()
	Assert.Equals("uuid-1", thread.replyRequest().ClientMessageId)
}

func (test *ThreadServiceTest) TestRetryDoesNothingUnlessFailed() {
	thread := test.opened("root-1")
	thread.Retry()
	Assert.Equals("", thread.SendStatus)
}

func (test *ThreadServiceTest) TestWriteNewTextAfterFailureStartsANewReply() {
	thread := test.sent("offline")
	thread.Write("Something else")
	Assert.Equals("", thread.SendStatus)
	Assert.Equals("", thread.pendingId)
}

func (test *ThreadServiceTest) TestSendAfterTheSessionEndedShowsTheSessionMessage() {
	thread := test.replying("Thanks")
	thread.finish(ThreadSendResult{}, &BehaviorError{Status: 401})
	Assert.Equals("session", thread.SendStatus)
	Assert.Equals("Thanks", thread.Draft)
	Assert.Equals("Reply not sent", thread.announce.Message)
}

func (test *ThreadServiceTest) TestSendIgnoresAnAnswerForAnotherReply() {
	thread := test.replying("Thanks")
	request := thread.replyRequest()
	request.ClientMessageId = "old"
	thread.Send(ThreadReplyRequested{Request: request})
	Assert.Equals("sending", thread.SendStatus)
}
