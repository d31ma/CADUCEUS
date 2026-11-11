// @Test(ThreadTestSetup, {"partition":"tests"})
type ThreadRemoteServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ThreadRemoteServiceTest) remote(expiresAtMs float64, sessionKey string) *ThreadRemoteService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	BrowserFakes{}.Answer("secureRandom", "uuid", "uuid-7")
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

func (test *ThreadRemoteServiceTest) TestPageSendsTheBearerTokenAndTheRoot() {
	page, failure := test.remote(3600000, "key-1").Page(ThreadPageRequest{ConversationId: "conv-1", RootId: "root-1"})
	Assert.Equals(nil, failure)
	Assert.Equals(2, len(page.Items))
	Assert.Equals("c-2", page.NextCursor)
}

func (test *ThreadRemoteServiceTest) TestPageSendsTheCursorForOlderReplies() {
	page, _ := test.remote(3600000, "key-1").Page(ThreadPageRequest{ConversationId: "conv-1", RootId: "root-1", Cursor: "c-2"})
	Assert.Equals("reply-1", page.Items[0].MessageId)
}

func (test *ThreadRemoteServiceTest) TestPageAfterTheSessionEndedIsUnauthorized() {
	_, failure := test.remote(0, "ended").Page(ThreadPageRequest{ConversationId: "conv-1", RootId: "root-1"})
	Assert.Equals(401, BehaviorErrorStatus(failure))
}

func (test *ThreadRemoteServiceTest) TestMessageLoadsTheRoot() {
	root, failure := test.remote(3600000, "key-1").Message("root-1")
	Assert.Equals(nil, failure)
	Assert.Equals("Build 42 is green.", root.Body)
	Assert.Equals(3, root.ReplyCount)
}

func (test *ThreadRemoteServiceTest) TestMessageAfterTheSessionEndedIsUnauthorized() {
	_, failure := test.remote(0, "ended").Message("root-1")
	Assert.Equals(401, BehaviorErrorStatus(failure))
}

func (test *ThreadRemoteServiceTest) TestSendPostsTheReply() {
	request := ThreadReplyRequest{ClientMessageId: "uuid-7", ConversationId: "conv-1", ThreadRootId: "root-1", AlsoToConversation: true, Body: "Thanks", AttachmentIds: []string{}}
	result, failure := test.remote(3600000, "key-1").Send(request)
	Assert.Equals(nil, failure)
	Assert.Equals("msg-uuid-7", result.Message.MessageId)
	Assert.Equals("root-1", result.Message.ThreadRootId)
	Assert.IsTrue(result.Message.AlsoToConversation)
}

func (test *ThreadRemoteServiceTest) TestSendAfterTheSessionEndedIsUnauthorized() {
	_, failure := test.remote(0, "ended").Send(ThreadReplyRequest{Body: "Thanks"})
	Assert.Equals(401, BehaviorErrorStatus(failure))
}

func (test *ThreadRemoteServiceTest) TestNewIdIsAUuid() {
	Assert.Equals("uuid-7", test.remote(3600000, "key-1").NewId())
}

func (test *ThreadRemoteServiceTest) TestMyNameIsTheCurrentAccountName() {
	Assert.Equals("Ana Ruiz", test.remote(3600000, "key-1").MyName())
}

func (test *ThreadRemoteServiceTest) TestPageRenewsOnceAfterA401AndTriesAgain() {
	remote := test.remote(3600000, "key-1")
	page, failure := remote.pageAfterRenew(ThreadMessagePage{}, &BehaviorError{Status: 401}, ThreadPageRequest{ConversationId: "conv-1", RootId: "root-1"})
	token, _ := remote.session.AccessToken()
	Assert.Equals(nil, failure)
	Assert.Equals(2, len(page.Items))
	Assert.Equals("access-2", token)
}

func (test *ThreadRemoteServiceTest) TestPageKeepsOtherFailuresWithoutRenewing() {
	remote := test.remote(3600000, "key-1")
	_, failure := remote.pageAfterRenew(ThreadMessagePage{}, &BehaviorError{Status: 503}, ThreadPageRequest{ConversationId: "conv-1", RootId: "root-1"})
	token, _ := remote.session.AccessToken()
	Assert.Equals(503, BehaviorErrorStatus(failure))
	Assert.Equals("access-1", token)
}

func (test *ThreadRemoteServiceTest) TestMessageRenewsOnceAfterA401AndTriesAgain() {
	root, failure := test.remote(3600000, "key-1").messageAfterRenew(ThreadMessage{}, &BehaviorError{Status: 401}, "root-1")
	Assert.Equals(nil, failure)
	Assert.Equals("root-1", root.MessageId)
}

func (test *ThreadRemoteServiceTest) TestSendRenewsOnceAfterA401AndTriesAgain() {
	request := ThreadReplyRequest{ClientMessageId: "uuid-7", ConversationId: "conv-1", ThreadRootId: "root-1", Body: "Thanks", AttachmentIds: []string{}}
	result, failure := test.remote(3600000, "key-1").sendAfterRenew(ThreadSendResult{}, &BehaviorError{Status: 401}, request)
	Assert.Equals(nil, failure)
	Assert.Equals("msg-uuid-7", result.Message.MessageId)
}

func (test *ThreadRemoteServiceTest) TestSecondFailureAfterRenewIsUnauthorized() {
	_, failure := test.remote(3600000, "ended").pageAfterRenew(ThreadMessagePage{}, &BehaviorError{Status: 401}, ThreadPageRequest{ConversationId: "conv-1", RootId: "root-1"})
	Assert.Equals(401, BehaviorErrorStatus(failure))
}
