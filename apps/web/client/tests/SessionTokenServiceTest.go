// @Test(SessionTestSetup, {"partition":"tests"})
type SessionTokenServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionTokenServiceTest) tokens() *SessionTokenService {
	client := NewIdentityClientWith(test.behaviors)
	return &SessionTokenService{identity: client}
}

func (test *SessionTokenServiceTest) pending() SessionPending {
	return SessionPending{DomainId: "example.com", Email: "ana@example.com", State: "state-1", Verifier: "verifier-1"}
}

func (test *SessionTokenServiceTest) TestExchangeKeepsTheTokensInMemory() {
	BrowserFakes{}.Answer("clock", "now", 1000)
	tokens := test.tokens()
	Assert.Equals(nil, tokens.Exchange(test.pending(), "good-code", "https://mesail.example.test/"))
	accessToken, failure := tokens.AccessToken("example.com")
	Assert.Equals(nil, failure)
	Assert.Equals("access-1", accessToken)
	Assert.Equals("id-1", tokens.IdToken("example.com"))
}

func (test *SessionTokenServiceTest) TestExchangeReportsARefusedCode() {
	BrowserFakes{}.Answer("clock", "now", 1000)
	failure := test.tokens().Exchange(test.pending(), "used-code", "https://mesail.example.test/")
	Assert.IsTrue(failure != nil)
}

func (test *SessionTokenServiceTest) TestAccessTokenRefreshesShortlyBeforeExpiry() {
	BrowserFakes{}.Answer("clock", "now", 1000)
	tokens := test.tokens()
	tokens.Exchange(test.pending(), "good-code", "https://mesail.example.test/")
	BrowserFakes{}.Answer("clock", "now", 50000)
	accessToken, _ := tokens.AccessToken("example.com")
	Assert.Equals("access-2", accessToken)
}

func (test *SessionTokenServiceTest) TestAccessTokenKeepsAFreshToken() {
	BrowserFakes{}.Answer("clock", "now", 1000)
	tokens := test.tokens()
	tokens.Exchange(test.pending(), "good-code", "https://mesail.example.test/")
	BrowserFakes{}.Answer("clock", "now", 40000)
	accessToken, _ := tokens.AccessToken("example.com")
	Assert.Equals("access-1", accessToken)
}

func (test *SessionTokenServiceTest) TestAccessTokenForgetsTokensWhenTheRefreshFails() {
	BrowserFakes{}.Answer("clock", "now", 1000)
	tokens := test.tokens()
	_, failure := tokens.AccessToken("example.org")
	Assert.IsTrue(failure != nil)
	Assert.Equals("", tokens.IdToken("example.org"))
}

func (test *SessionTokenServiceTest) TestForgetDropsTheTokens() {
	BrowserFakes{}.Answer("clock", "now", 1000)
	tokens := test.tokens()
	tokens.Exchange(test.pending(), "good-code", "https://mesail.example.test/")
	tokens.Forget("example.com")
	Assert.Equals("", tokens.IdToken("example.com"))
}
