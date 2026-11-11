// @Test(ThreadTestSetup, {"partition":"tests"})
type ThreadRemoteServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ThreadRemoteServiceTest) remote(expiresAtMs float64) *ThreadRemoteService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	BrowserFakes{}.Answer("secureRandom", "uuid", "uuid-7")
	tokens := &SessionTokenService{
		identity: NewIdentityClientWith(test.behaviors),
		keys:     map[string]string{"example.com": "ended"},
		tokens:   map[string]SessionTokens{"example.com": {AccessToken: "access-1", ExpiresAtMs: expiresAtMs}},
	}
	accounts := &SessionAccountService{}
	accounts.Add(Me{MemberId: "m-1", DomainId: "example.com", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com", Roles: []string{}})
	return &ThreadRemoteService{client: NewThreadClientWith(test.behaviors), tokens: tokens, accounts: accounts}
}

func (test *ThreadRemoteServiceTest) TestPageSendsTheBearerTokenAndTheRoot() {
	page, failure := test.remote(3600000).Page(ThreadPageRequest{ConversationId: "conv-1", RootId: "root-1"})
	Assert.Equals(nil, failure)
	Assert.Equals(2, len(page.Items))
	Assert.Equals("c-2", page.NextCursor)
}

func (test *ThreadRemoteServiceTest) TestPageSendsTheCursorForOlderReplies() {
	page, _ := test.remote(3600000).Page(ThreadPageRequest{ConversationId: "conv-1", RootId: "root-1", Cursor: "c-2"})
	Assert.Equals("reply-1", page.Items[0].MessageId)
}

func (test *ThreadRemoteServiceTest) TestPageAfterTheSessionEndedIsUnauthorized() {
	_, failure := test.remote(0).Page(ThreadPageRequest{ConversationId: "conv-1", RootId: "root-1"})
	Assert.Equals(401, BehaviorErrorStatus(failure))
}

func (test *ThreadRemoteServiceTest) TestMessageLoadsTheRoot() {
	root, failure := test.remote(3600000).Message("root-1")
	Assert.Equals(nil, failure)
	Assert.Equals("Build 42 is green.", root.Body)
	Assert.Equals(3, root.ReplyCount)
}

func (test *ThreadRemoteServiceTest) TestMessageAfterTheSessionEndedIsUnauthorized() {
	_, failure := test.remote(0).Message("root-1")
	Assert.Equals(401, BehaviorErrorStatus(failure))
}

func (test *ThreadRemoteServiceTest) TestSendPostsTheReply() {
	request := ThreadReplyRequest{ClientMessageId: "uuid-7", ConversationId: "conv-1", ThreadRootId: "root-1", AlsoToConversation: true, Body: "Thanks", AttachmentIds: []string{}}
	result, failure := test.remote(3600000).Send(request)
	Assert.Equals(nil, failure)
	Assert.Equals("msg-uuid-7", result.Message.MessageId)
	Assert.Equals("root-1", result.Message.ThreadRootId)
	Assert.IsTrue(result.Message.AlsoToConversation)
}

func (test *ThreadRemoteServiceTest) TestSendAfterTheSessionEndedIsUnauthorized() {
	_, failure := test.remote(0).Send(ThreadReplyRequest{Body: "Thanks"})
	Assert.Equals(401, BehaviorErrorStatus(failure))
}

func (test *ThreadRemoteServiceTest) TestNewIdIsAUuid() {
	Assert.Equals("uuid-7", test.remote(3600000).NewId())
}

func (test *ThreadRemoteServiceTest) TestMyNameIsTheCurrentAccountName() {
	Assert.Equals("Ana Ruiz", test.remote(3600000).MyName())
}
