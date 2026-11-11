// @Test(ConversationTestSetup, {"partition":"tests"})
type RealtimeServiceTest struct {
	behaviors *BehaviorContext
}

func (test *RealtimeServiceTest) realtime(accessToken string) *RealtimeService {
	return test.realtimeAt(accessToken, 100000000)
}

func (test *RealtimeServiceTest) realtimeAt(accessToken string, expiresAtMs float64) *RealtimeService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	accounts := &SessionAccountService{}
	accounts.Add(Me{MemberId: "ana", DomainId: "example.com", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com"})
	tokens := &SessionTokenService{tokens: map[string]SessionTokens{"example.com": {AccessToken: accessToken, ExpiresAtMs: expiresAtMs}}, identity: NewIdentityClientWith(test.behaviors)}
	service := &RealtimeService{realtimeClient: &RealtimeClient{}, token: &ConversationTokenService{session: &SessionService{tokens: tokens, accounts: accounts}, accounts: accounts}}
	service.Start()
	return service
}

func (test *RealtimeServiceTest) TestStartWaitsForTheServer() {
	Assert.Equals("connecting", test.realtime("access-1").State)
}

func (test *RealtimeServiceTest) TestHelloWithoutAnOpenSocketGoesOffline() {
	realtime := test.realtime("access-1")
	realtime.Received(RealtimeEvent{Type: "hello"})
	Assert.Equals("offline", realtime.State)
}

func (test *RealtimeServiceTest) TestReauthWithoutAnOpenSocketGoesOffline() {
	realtime := test.realtime("access-1")
	realtime.Received(RealtimeEvent{Type: "reauth"})
	Assert.Equals("offline", realtime.State)
}

func (test *RealtimeServiceTest) TestOtherEventsKeepTheState() {
	realtime := test.realtime("access-1")
	realtime.Received(RealtimeEvent{Type: "message.created", ConversationId: "cnv_general"})
	Assert.Equals("connecting", realtime.State)
}

func (test *RealtimeServiceTest) TestDomainChangedWithoutAnOpenSocketGoesOffline() {
	realtime := test.realtime("access-1")
	realtime.DomainChanged(SessionDomainChanged{DomainId: "example.com"})
	Assert.Equals("offline", realtime.State)
}

func (test *RealtimeServiceTest) TestHelloWithAnEndedSessionGoesOffline() {
	realtime := test.realtimeAt("access-1", 0)
	realtime.Received(RealtimeEvent{Type: "hello"})
	Assert.Equals("offline", realtime.State)
}

func (test *RealtimeServiceTest) TestAcceptedAuthenticationIsLive() {
	realtime := test.realtime("access-1")
	realtime.Received(RealtimeEvent{Type: "auth.accepted", State: "2026-11-10T14:06:00Z"})
	Assert.Equals("live", realtime.State)
}

func (test *RealtimeServiceTest) TestExpiredAuthenticationIsAnsweredAgain() {
	realtime := test.realtime("access-1")
	realtime.Received(RealtimeEvent{Type: "auth.accepted"})
	realtime.Received(RealtimeEvent{Type: "auth.expired", State: "4401"})
	Assert.IsTrue(realtime.State != "live")
}
