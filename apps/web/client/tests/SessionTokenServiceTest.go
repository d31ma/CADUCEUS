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
	tokens.SignIn(SessionSignInRequest{Login: "ana", Password: "right"})
	return tokens
}

func (test *SessionTokenServiceTest) TestSignInKeepsTheTokenInMemoryAndTheKey() {
	tokens := test.tokens()
	session, failure := tokens.SignIn(SessionSignInRequest{Login: "ana", Password: "right"})
	accessToken, _ := tokens.AccessToken()
	Assert.Equals(nil, failure)
	Assert.Equals("example.com", session.Me.DomainId)
	Assert.Equals(2, len(session.Domains))
	Assert.Equals("access-1", accessToken)
	Assert.Equals("key-1", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestSignInSendsTheOneTimeCode() {
	_, failure := test.tokens().SignIn(SessionSignInRequest{Login: "ana", Password: "right", OneTimeCode: "123456"})
	Assert.Equals(nil, failure)
}

func (test *SessionTokenServiceTest) TestSignInWithAnAddressOpensThatDomain() {
	session, _ := test.tokens().SignIn(SessionSignInRequest{Login: "ana@example.org", Password: "right"})
	Assert.Equals("example.org", session.Me.DomainId)
}

func (test *SessionTokenServiceTest) TestSignInKeepsNothingWhenRefused() {
	tokens := test.tokens()
	_, failure := tokens.SignIn(SessionSignInRequest{Login: "ana", Password: "wrong"})
	Assert.IsTrue(failure != nil)
	Assert.Equals("", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestSignInRefusalKeepsItsStatus() {
	_, failure := test.tokens().SignIn(SessionSignInRequest{Login: "ana@partner.test", Password: "right"})
	Assert.Equals(403, BehaviorErrorStatus(failure))
}

func (test *SessionTokenServiceTest) TestAccessTokenRefreshesShortlyBeforeExpiry() {
	tokens := test.signedIn()
	BrowserFakes{}.Answer("clock", "now", 50000)
	accessToken, _ := tokens.AccessToken()
	Assert.Equals("access-2", accessToken)
	Assert.Equals("key-2", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestAccessTokenKeepsAFreshToken() {
	tokens := test.signedIn()
	BrowserFakes{}.Answer("clock", "now", 40000)
	accessToken, _ := tokens.AccessToken()
	Assert.Equals("access-1", accessToken)
}

func (test *SessionTokenServiceTest) TestAccessTokenForgetsAnEndedSession() {
	tokens := test.signedIn()
	tokens.key = "ended"
	BrowserFakes{}.Answer("clock", "now", 50000)
	_, failure := tokens.AccessToken()
	Assert.IsTrue(failure != nil)
	Assert.Equals("", tokens.SavedKey())
	Assert.Equals("", tokens.Current().Me.DomainId)
}

func (test *SessionTokenServiceTest) TestRefreshOpensTheDefaultDomainWhenTheOpenOneWasRemoved() {
	tokens := test.signedIn()
	tokens.key = "key-removed"
	BrowserFakes{}.Answer("clock", "now", 50000)
	accessToken, failure := tokens.AccessToken()
	Assert.Equals(nil, failure)
	Assert.Equals("access-back", accessToken)
	Assert.Equals("example.com", tokens.Current().Me.DomainId)
	Assert.Equals("key-back", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestRefreshEndsTheSessionWhenNoDomainOpens() {
	tokens := test.signedIn()
	tokens.key = "key-cut"
	BrowserFakes{}.Answer("clock", "now", 50000)
	_, failure := tokens.AccessToken()
	Assert.IsTrue(failure != nil)
	Assert.Equals("", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestRestoreRefreshesTheStoredSession() {
	session, failure := test.tokens().Restore("key-1")
	Assert.Equals(nil, failure)
	Assert.Equals("example.com", session.Me.DomainId)
}

func (test *SessionTokenServiceTest) TestRestoreWithoutAKeyFindsNobody() {
	tokens := test.tokens()
	_, failure := tokens.Restore("")
	Assert.IsTrue(failure != nil)
	Assert.Equals("", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestSwitchOpensTheChosenDomain() {
	tokens := test.signedIn()
	session, failure := tokens.Switch("example.org")
	accessToken, _ := tokens.AccessToken()
	Assert.Equals(nil, failure)
	Assert.Equals("example.org", session.Me.DomainId)
	Assert.Equals("access-org", accessToken)
	Assert.Equals("key-org", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestSwitchToARemovedDomainKeepsTheSession() {
	tokens := test.signedIn()
	_, failure := tokens.Switch("gone.example")
	Assert.Equals(404, BehaviorErrorStatus(failure))
	Assert.Equals("key-1", tokens.SavedKey())
	Assert.Equals("example.com", tokens.Current().Me.DomainId)
}

func (test *SessionTokenServiceTest) TestSwitchWaitsForNoRunningRefresh() {
	tokens := test.signedIn()
	tokens.refreshing = true
	_, failure := tokens.Switch("example.org")
	Assert.IsTrue(failure != nil)
	Assert.Equals("key-1", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestSignOutForgetsTheSession() {
	tokens := test.signedIn()
	tokens.SignOut()
	Assert.Equals("", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestChangePasswordKeepsTheFreshSession() {
	tokens := test.signedIn()
	_, failure := tokens.ChangePassword(SessionPasswordChange{CurrentPassword: "right", NewPassword: "a-long-new-password"})
	accessToken, _ := tokens.AccessToken()
	Assert.Equals(nil, failure)
	Assert.Equals("access-new", accessToken)
	Assert.Equals("key-new", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestChangePasswordKeepsTheOldSessionWhenRefused() {
	tokens := test.signedIn()
	_, failure := tokens.ChangePassword(SessionPasswordChange{CurrentPassword: "wrong", NewPassword: "a-long-new-password"})
	Assert.IsTrue(failure != nil)
	Assert.Equals("key-1", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestRenewRefreshesNow() {
	tokens := test.signedIn()
	Assert.Equals(nil, tokens.Renew())
	accessToken, _ := tokens.AccessToken()
	Assert.Equals("access-2", accessToken)
}

func (test *SessionTokenServiceTest) TestRenewSkipsWhileARefreshIsRunning() {
	tokens := test.signedIn()
	tokens.refreshing = true
	Assert.Equals(nil, tokens.Renew())
	Assert.Equals("key-1", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestAccessTokenUsesTheCurrentTokenWhileARefreshIsRunning() {
	tokens := test.signedIn()
	tokens.refreshing = true
	BrowserFakes{}.Answer("clock", "now", 50000)
	accessToken, failure := tokens.AccessToken()
	Assert.Equals(nil, failure)
	Assert.Equals("access-1", accessToken)
	Assert.Equals("key-1", tokens.SavedKey())
}

func (test *SessionTokenServiceTest) TestRefreshClearsTheRunningMarkWhenDone() {
	tokens := test.signedIn()
	BrowserFakes{}.Answer("clock", "now", 50000)
	tokens.AccessToken()
	Assert.Equals(false, tokens.refreshing)
}

func (test *SessionTokenServiceTest) TestRefreshClearsTheRunningMarkWhenRefused() {
	tokens := test.signedIn()
	tokens.key = "ended"
	Assert.IsTrue(tokens.Renew() != nil)
	Assert.Equals(false, tokens.refreshing)
}
