// @Test(ThreadTestSetup, {"partition":"tests"})
type ThreadServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ThreadServiceTest) remote(expiresAtMs float64, sessionKey string) *ThreadRemoteService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	BrowserFakes{}.Answer("secureRandom", "uuid", "uuid-1")
	tokens := &SessionTokenService{
		identity: NewIdentityClientWith(test.behaviors),
		keys:     map[string]string{"example.com": sessionKey},
		tokens:   map[string]SessionTokens{"example.com": {AccessToken: "access-1", ExpiresAtMs: expiresAtMs}},
	}
	accounts := &SessionAccountService{}
	accounts.Add(Me{MemberId: "m-1", DomainId: "example.com", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com", Roles: []string{}})
	session := &SessionService{realm: &SessionRealmService{}, tokens: tokens, accounts: accounts, problems: &SessionProblemService{}, Step: "signedIn"}
	return &ThreadRemoteService{client: NewThreadClientWith(test.behaviors), session: session, accounts: accounts}
}

func (test *ThreadServiceTest) rows() *ThreadRowService {
	BrowserFakes{}.Answer("clock", "timeZone", "UTC")
	return &ThreadRowService{times: &ConversationTimeService{}, reactions: &ConversationReactionService{}}
}

func (test *ThreadServiceTest) threadWith(remote *ThreadRemoteService) *ThreadService {
	shell := &ShellService{}
	shell.Start()
	return &ThreadService{shell: shell, announce: &ShellAnnounceService{}, remote: remote, rows: test.rows()}
}

func (test *ThreadServiceTest) thread() *ThreadService {
	return test.threadWith(test.remote(3600000, "key-1"))
}

func (test *ThreadServiceTest) opened(rootId string) *ThreadService {
	thread := test.thread()
	thread.OpenMessage("conv-1", rootId)
	return thread
}

func (test *ThreadServiceTest) sent(text string) *ThreadService {
	thread := test.opened("root-1")
	thread.ReplyWith(text, false)
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
	thread := test.opened("root-1")
	Assert.IsTrue(thread.shell.ThreadOpen)
	Assert.Equals("thread", thread.shell.Pane)
	Assert.Equals("root-1", thread.RootId)
	Assert.Equals("conv-1", thread.ConversationId)
}

func (test *ThreadServiceTest) TestOpenMessageShowsRepliesOldestFirst() {
	thread := test.opened("root-1")
	Assert.Equals("ready", thread.Status)
	Assert.Equals([]string{"reply-2", "reply-3"}, test.replyIds(thread))
	Assert.Equals("c-2", thread.NextCursor)
}

func (test *ThreadServiceTest) TestOpenMessageShowsTheRootFirstInTheTimeline() {
	thread := test.opened("root-1")
	Assert.Equals(3, len(thread.Timeline))
	Assert.Equals("root-1", thread.Timeline[0].MessageId)
	Assert.IsTrue(thread.Timeline[0].IsRoot)
	Assert.IsTrue(!thread.Timeline[1].IsRoot)
}

func (test *ThreadServiceTest) TestOpenMessageShowsTheRootFromItsOwnCall() {
	thread := test.opened("root-1")
	Assert.IsTrue(thread.HasRoot)
	Assert.Equals("Ana Ruiz", thread.Root.AuthorName)
	Assert.Equals("09:12", thread.Root.Time)
}

func (test *ThreadServiceTest) TestOpenMessageCountsRepliesFromTheRoot() {
	Assert.Equals("3 replies", test.opened("root-1").RepliesLabel)
}

func (test *ThreadServiceTest) TestShowOlderAddsOlderRepliesFirst() {
	thread := test.opened("root-1")
	thread.ShowOlder()
	Assert.Equals([]string{"reply-1", "reply-2", "reply-3"}, test.replyIds(thread))
	Assert.Equals("", thread.NextCursor)
	Assert.IsTrue(!thread.LoadingOlder)
}

func (test *ThreadServiceTest) TestOpenMessageFailsWhenTheRootDoesNotLoad() {
	Assert.Equals("error", test.opened("root-2").Status)
}

func (test *ThreadServiceTest) TestARemovedRootSaysItIsGone() {
	thread := test.opened("root-1")
	thread.applyPage(MessagePage{}, &BehaviorError{Status: 404})
	Assert.Equals("gone", thread.Status)
}

func (test *ThreadServiceTest) TestOpenMessageFailureShowsTheErrorState() {
	thread := test.opened("broken")
	Assert.Equals("error", thread.Status)
	Assert.Equals(0, len(thread.Replies))
}

func (test *ThreadServiceTest) TestOpenMessageAfterTheSessionEndedShowsTheSessionState() {
	thread := test.threadWith(test.remote(0, "ended"))
	thread.OpenMessage("conv-1", "root-1")
	Assert.Equals("session", thread.Status)
}

func (test *ThreadServiceTest) TestReloadLoadsAgain() {
	thread := test.opened("root-1")
	thread.Replies = []ThreadRow{}
	thread.Reload()
	Assert.Equals("ready", thread.Status)
	Assert.Equals(2, len(thread.Replies))
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

func (test *ThreadServiceTest) TestRefreshLoadsTheOpenThreadAgain() {
	thread := test.opened("root-1")
	thread.Replies = []ThreadRow{}
	thread.Refresh(ThreadRefreshRequested{ConversationId: "conv-1"})
	Assert.Equals(2, len(thread.Replies))
}

func (test *ThreadServiceTest) TestRefreshIgnoresAnotherConversation() {
	thread := test.opened("root-1")
	thread.Replies = []ThreadRow{}
	thread.Refresh(ThreadRefreshRequested{ConversationId: "conv-2"})
	Assert.Equals(0, len(thread.Replies))
}

func (test *ThreadServiceTest) TestRefreshDoesNothingWhenClosed() {
	thread := test.thread()
	thread.Refresh(ThreadRefreshRequested{ConversationId: "conv-1"})
	Assert.Equals("", thread.Status)
}

func (test *ThreadServiceTest) TestShownLoadsTheOpenThreadAgain() {
	thread := test.opened("root-1")
	thread.Replies = []ThreadRow{}
	thread.Shown(VisibilityChanged{Visible: true})
	Assert.Equals(2, len(thread.Replies))
}

func (test *ThreadServiceTest) TestShownIgnoresAHiddenPage() {
	thread := test.thread()
	thread.Shown(VisibilityChanged{Visible: false})
	Assert.Equals("", thread.Status)
}

func (test *ThreadServiceTest) TestDomainChangeClosesTheThread() {
	thread := test.opened("root-1")
	thread.DomainChanged(SessionDomainChanged{DomainId: "example.org"})
	Assert.IsTrue(!thread.shell.ThreadOpen)
	Assert.Equals("", thread.RootId)
	Assert.Equals(0, len(thread.Replies))
}

func (test *ThreadServiceTest) TestReplyWithAnEmptyDraftSendsNothing() {
	thread := test.opened("root-1")
	thread.ReplyWith("   ", true)
	Assert.Equals("", thread.SendStatus)
	Assert.Equals(2, len(thread.Replies))
}

func (test *ThreadServiceTest) TestReplyWhileSendingSendsNothingMore() {
	thread := test.opened("root-1")
	thread.SendStatus = "sending"
	thread.ReplyWith("More", false)
	Assert.Equals(2, len(thread.Replies))
}

func (test *ThreadServiceTest) TestReplyWithSendsAThreadReply() {
	thread := test.sent("  Thanks  ")
	Assert.Equals([]string{"reply-2", "reply-3", "msg-uuid-1"}, test.replyIds(thread))
	Assert.Equals("Thanks", thread.Replies[2].Body)
	Assert.Equals("", thread.SendStatus)
	Assert.Equals("", thread.FailedDraft)
	Assert.Equals("Reply sent", thread.announce.Message)
}

func (test *ThreadServiceTest) TestReplyAsksForAThreadReply() {
	request := test.sent("offline").replyRequest()
	Assert.Equals("uuid-1", request.ClientMessageId)
	Assert.Equals("conv-1", request.ConversationId)
	Assert.Equals("root-1", request.ThreadRootId)
	Assert.Equals("offline", request.Body)
	Assert.Equals(0, len(request.AttachmentIds))
}

func (test *ThreadServiceTest) TestReplyAlsoSendsToTheConversationWhenTicked() {
	thread := test.opened("root-1")
	thread.ReplyWith("offline", true)
	Assert.IsTrue(thread.replyRequest().AlsoToConversation)
	Assert.IsTrue(thread.AlsoToConversation)
}

func (test *ThreadServiceTest) TestReplyStaysInTheThreadByDefault() {
	Assert.IsTrue(!test.sent("offline").replyRequest().AlsoToConversation)
}

func (test *ThreadServiceTest) TestSendResetsAlsoToConversation() {
	thread := test.opened("root-1")
	thread.ReplyWith("Thanks", true)
	Assert.IsTrue(!thread.AlsoToConversation)
	Assert.Equals("msg-uuid-1", thread.Replies[2].MessageId)
}

func (test *ThreadServiceTest) TestSendFailureKeepsTheDraft() {
	thread := test.sent("offline")
	Assert.Equals("failed", thread.SendStatus)
	Assert.Equals("offline", thread.FailedDraft)
	Assert.Equals("offline", thread.Pending.Body)
	Assert.Equals("Ana Ruiz", thread.Pending.AuthorName)
	Assert.Equals("Reply not sent", thread.announce.Message)
}

func (test *ThreadServiceTest) TestRetryKeepsTheSameClientMessageId() {
	thread := test.sent("offline")
	BrowserFakes{}.Answer("secureRandom", "uuid", "uuid-2")
	thread.Retry()
	Assert.Equals("failed", thread.SendStatus)
	Assert.Equals("uuid-1", thread.replyRequest().ClientMessageId)
}

func (test *ThreadServiceTest) TestSendAgainAfterFailureKeepsTheSameClientMessageId() {
	thread := test.sent("offline")
	BrowserFakes{}.Answer("secureRandom", "uuid", "uuid-2")
	thread.ReplyWith("offline", false)
	Assert.Equals("uuid-1", thread.replyRequest().ClientMessageId)
}

func (test *ThreadServiceTest) TestRetryDoesNothingUnlessFailed() {
	thread := test.opened("root-1")
	thread.Retry()
	Assert.Equals("", thread.SendStatus)
}

func (test *ThreadServiceTest) TestReplyWithNewTextAfterFailureStartsANewReply() {
	thread := test.sent("offline")
	BrowserFakes{}.Answer("secureRandom", "uuid", "uuid-2")
	thread.ReplyWith("Something else", false)
	Assert.Equals("msg-uuid-2", thread.Replies[2].MessageId)
}

func (test *ThreadServiceTest) TestSendAfterTheSessionEndedShowsTheSessionMessage() {
	thread := test.sent("offline")
	thread.finish(ThreadSendResult{}, &BehaviorError{Status: 401})
	Assert.Equals("session", thread.SendStatus)
	Assert.Equals("offline", thread.FailedDraft)
	Assert.Equals("Reply not sent", thread.announce.Message)
}
