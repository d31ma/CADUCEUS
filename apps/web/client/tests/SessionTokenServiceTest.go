// @Test(SessionTestSetup, {"partition":"tests"})
type SessionTokenServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionTokenServiceTest) tokens() *SessionTokenService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	return &SessionTokenService{identity: NewIdentityClientWith(test.behaviors)}
}

func (test *SessionTokenServiceTest) signedIn() *SessionTokenService {
	tokens := test.tokens()
	tokens.SignIn(SessionSignInRequest{Address: "ana@example.com", Password: "right"})
	return tokens
}

func (test *SessionTokenServiceTest) TestSignInKeepsTheTokenInMemoryAndTheKeyInSessionState() {
	tokens := test.tokens()
	me, failure := tokens.SignIn(SessionSignInRequest{Address: "ana@example.com", Password: "right"})
	accessToken, _ := tokens.AccessToken("example.com")
	Assert.Equals(nil, failure)
	Assert.Equals("example.com", me.DomainId)
	Assert.Equals("access-1", accessToken)
	Assert.Equals(map[string]string{"example.com": "key-1"}, tokens.SavedKeys())
}

func (test *SessionTokenServiceTest) TestSignInSendsTheOneTimeCode() {
	_, failure := test.tokens().SignIn(SessionSignInRequest{Address: "ana@example.com", Password: "right", OneTimeCode: "123456"})
	Assert.Equals(nil, failure)
}

func (test *SessionTokenServiceTest) TestSignInKeepsNothingWhenRefused() {
	tokens := test.tokens()
	_, failure := tokens.SignIn(SessionSignInRequest{Address: "ana@example.com", Password: "wrong"})
	Assert.IsTrue(failure != nil)
	Assert.Equals(0, len(tokens.SavedKeys()))
}

func (test *SessionTokenServiceTest) TestAccessTokenRefreshesShortlyBeforeExpiry() {
	tokens := test.signedIn()
	BrowserFakes{}.Answer("clock", "now", 50000)
	accessToken, _ := tokens.AccessToken("example.com")
	Assert.Equals("access-2", accessToken)
	Assert.Equals("key-2", tokens.SavedKeys()["example.com"])
}

func (test *SessionTokenServiceTest) TestAccessTokenKeepsAFreshToken() {
	tokens := test.signedIn()
	BrowserFakes{}.Answer("clock", "now", 40000)
	accessToken, _ := tokens.AccessToken("example.com")
	Assert.Equals("access-1", accessToken)
}

func (test *SessionTokenServiceTest) TestAccessTokenForgetsAnEndedSession() {
	tokens := test.signedIn()
	tokens.keys["example.com"] = "ended"
	BrowserFakes{}.Answer("clock", "now", 50000)
	_, failure := tokens.AccessToken("example.com")
	Assert.IsTrue(failure != nil)
	Assert.Equals(0, len(tokens.SavedKeys()))
}

func (test *SessionTokenServiceTest) TestRestoreRefreshesEveryStoredSession() {
	tokens := test.tokens()
	signedIn := tokens.Restore(map[string]string{"example.org": "key-org", "example.com": "key-1", "example.net": "ended"})
	Assert.Equals(2, len(signedIn))
	Assert.Equals("example.com", signedIn[0].DomainId)
	Assert.Equals("example.org", signedIn[1].DomainId)
	Assert.Equals(map[string]string{"example.com": "key-2", "example.org": "key-org-2"}, tokens.SavedKeys())
}

func (test *SessionTokenServiceTest) TestRestoreWithoutKeysFindsNobody() {
	Assert.Equals(0, len(test.tokens().Restore(nil)))
}

func (test *SessionTokenServiceTest) TestSignOutForgetsTheSession() {
	tokens := test.signedIn()
	Assert.Equals(nil, tokens.SignOut("example.com"))
	accessToken, _ := tokens.AccessToken("example.com")
	Assert.Equals("", accessToken)
	Assert.Equals(0, len(tokens.SavedKeys()))
}
