// @Test(SessionTestSetup, {"partition":"tests"})
type SessionServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionServiceTest) build(keys map[string]string) *SessionService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	client := NewIdentityClientWith(test.behaviors)
	return &SessionService{
		realm:       &SessionRealmService{},
		tokens:      &SessionTokenService{identity: client},
		accounts:    &SessionAccountService{},
		problems:    &SessionProblemService{},
		SessionKeys: keys,
	}
}

func (test *SessionServiceTest) started() *SessionService {
	session := test.build(nil)
	session.Start()
	session.Restore(SessionRestoreRequested{})
	return session
}

func (test *SessionServiceTest) signedIn() *SessionService {
	session := test.started()
	session.Email = "ana@example.com"
	session.Password = "right"
	session.Remember = true
	session.signIn()
	return session
}

func (test *SessionServiceTest) signedInTwice() *SessionService {
	session := test.signedIn()
	session.AddDomain()
	session.Email = "ana@example.org"
	session.Password = "right"
	session.signIn()
	return session
}

func (test *SessionServiceTest) TestStartWaitsForTheRestore() {
	session := test.build(nil)
	session.Start()
	Assert.Equals("", session.Step)
}

func (test *SessionServiceTest) TestStartWithoutSessionsShowsTheForm() {
	session := test.started()
	Assert.Equals("signIn", session.Step)
	Assert.IsTrue(!session.Remember)
}

func (test *SessionServiceTest) TestStartRestoresStoredSessions() {
	session := test.build(map[string]string{"example.com": "key-1"})
	session.Restore(SessionRestoreRequested{})
	Assert.Equals("signedIn", session.Step)
	Assert.Equals("example.com", session.accounts.CurrentDomain)
	Assert.Equals(map[string]string{"example.com": "key-2"}, session.SessionKeys)
}

func (test *SessionServiceTest) TestSignInSavesTheSessionKey() {
	Assert.Equals(map[string]string{"example.com": "key-1"}, test.signedIn().SessionKeys)
}

func (test *SessionServiceTest) TestStartWithAnEndedSessionShowsTheForm() {
	session := test.build(map[string]string{"example.com": "ended"})
	session.Restore(SessionRestoreRequested{})
	Assert.Equals("signIn", session.Step)
}

func (test *SessionServiceTest) TestSignInRefusesAnIncompleteEmail() {
	session := test.started()
	session.Email = "ana"
	session.Password = "right"
	session.signIn()
	Assert.Equals("signIn", session.Step)
	Assert.Equals("invalidEmail", session.Problem)
}

func (test *SessionServiceTest) TestSignInOpensTheShell() {
	session := test.signedIn()
	Assert.Equals("signedIn", session.Step)
	Assert.Equals("", session.Problem)
	Assert.Equals("example.com", session.accounts.CurrentDomain)
}

func (test *SessionServiceTest) TestSignInClearsThePasswordAndCode() {
	session := test.started()
	session.Email = "ana@example.com"
	session.Password = "right"
	session.OneTimeCode = "123456"
	session.signIn()
	Assert.Equals("", session.Password)
	Assert.Equals("", session.OneTimeCode)
}

func (test *SessionServiceTest) TestSignInRemembersOnlyTheDomainAndEmail() {
	session := test.signedIn()
	Assert.Equals([]SessionRecentDomain{{DomainId: "example.com", Email: "ana@example.com"}}, session.RecentDomains)
}

func (test *SessionServiceTest) TestSignInWithoutRememberKeepsNothing() {
	session := test.started()
	session.Email = "ana@example.com"
	session.Password = "right"
	session.signIn()
	Assert.Equals(0, len(session.RecentDomains))
}

func (test *SessionServiceTest) TestSignInRefusedShowsAProblem() {
	session := test.started()
	session.Email = "ana@example.com"
	session.Password = "wrong"
	session.Remember = true
	session.signIn()
	Assert.Equals("signIn", session.Step)
	Assert.Equals("unavailable", session.Problem)
	Assert.Equals("", session.Password)
	Assert.Equals(0, len(session.RecentDomains))
}

func (test *SessionServiceTest) TestUseRecentFillsTheEmail() {
	session := test.started()
	session.UseRecent(SessionRecentDomain{DomainId: "example.org", Email: "ana@example.org"})
	Assert.Equals("ana@example.org", session.Email)
	Assert.Equals("ana@example.org", session.ShownEmail)
}

func (test *SessionServiceTest) TestForgetRemovesOneRecentDomain() {
	session := test.signedIn()
	session.Forget("example.com")
	Assert.Equals(0, len(session.RecentDomains))
}

func (test *SessionServiceTest) TestForgetAllRemovesEveryRecentDomain() {
	session := test.signedIn()
	session.ForgetAll()
	Assert.Equals(0, len(session.RecentDomains))
}

func (test *SessionServiceTest) TestAddDomainShowsAnEmptyForm() {
	session := test.signedIn()
	session.AddDomain()
	Assert.Equals("signIn", session.Step)
	Assert.IsTrue(session.Adding)
	Assert.Equals("", session.Email)
}

func (test *SessionServiceTest) TestCancelAddReturnsToTheShell() {
	session := test.signedIn()
	session.AddDomain()
	session.CancelAdd()
	Assert.Equals("signedIn", session.Step)
	Assert.IsTrue(!session.Adding)
}

func (test *SessionServiceTest) TestSecondDomainBecomesCurrent() {
	session := test.signedInTwice()
	Assert.Equals(2, len(session.accounts.Accounts))
	Assert.Equals("example.org", session.accounts.CurrentDomain)
	Assert.IsTrue(!session.Adding)
}

func (test *SessionServiceTest) TestSwitchChangesTheCurrentDomain() {
	session := test.signedInTwice()
	session.Switch("example.com")
	Assert.Equals("example.com", session.accounts.CurrentDomain)
}

func (test *SessionServiceTest) TestSignOutKeepsTheOtherDomain() {
	session := test.signedInTwice()
	session.SignOut()
	Assert.Equals("signedIn", session.Step)
	Assert.Equals("example.org", session.SignedOutDomain)
	Assert.Equals("example.com", session.accounts.CurrentDomain)
	Assert.Equals(map[string]string{"example.com": "key-1"}, session.SessionKeys)
}

func (test *SessionServiceTest) TestSignOutOfTheLastDomainShowsSignedOut() {
	session := test.signedIn()
	session.SignOut()
	Assert.Equals("signedOut", session.Step)
	Assert.Equals("example.com", session.SignedOutDomain)
}

func (test *SessionServiceTest) TestSignOutAllEndsEverySession() {
	session := test.signedInTwice()
	session.SignOutAll()
	Assert.Equals("signedOut", session.Step)
	Assert.Equals("", session.SignedOutDomain)
	Assert.Equals(0, len(session.SessionKeys))
	Assert.Equals(0, len(session.accounts.Accounts))
}

func (test *SessionServiceTest) TestSignInAgainFillsTheCurrentAddress() {
	session := test.signedIn()
	session.Step = "expired"
	session.SignInAgain()
	Assert.Equals("signIn", session.Step)
	Assert.Equals("ana@example.com", session.Email)
}

func (test *SessionServiceTest) TestShownKeepsAWorkingSession() {
	session := test.signedIn()
	session.Shown(VisibilityChanged{Visible: true})
	Assert.Equals("signedIn", session.Step)
}

func (test *SessionServiceTest) TestShownEndsASessionThatCannotRefresh() {
	session := test.signedIn()
	session.tokens.keys["example.com"] = "ended"
	BrowserFakes{}.Answer("clock", "now", 50000)
	session.Shown(VisibilityChanged{Visible: true})
	Assert.Equals("expired", session.Step)
}

func (test *SessionServiceTest) TestShownIgnoresAHiddenPage() {
	session := test.signedIn()
	session.tokens.keys["example.com"] = "ended"
	BrowserFakes{}.Answer("clock", "now", 50000)
	session.Shown(VisibilityChanged{Visible: false})
	Assert.Equals("signedIn", session.Step)
}

func (test *SessionServiceTest) TestAccessTokenGivesTheCurrentDomainToken() {
	accessToken, failure := test.signedIn().AccessToken()
	Assert.Equals(nil, failure)
	Assert.Equals("access-1", accessToken)
}

func (test *SessionServiceTest) TestChangePasswordSavesTheNewKey() {
	session := test.signedIn()
	Assert.Equals(nil, session.ChangePassword(SessionPasswordChange{CurrentPassword: "right", NewPassword: "longer-and-better"}))
	Assert.Equals("key-new", session.SessionKeys["example.com"])
}

func (test *SessionServiceTest) TestRenewGivesAFreshTokenAndSavesTheKey() {
	session := test.signedIn()
	accessToken, _ := session.Renew()
	Assert.Equals("access-2", accessToken)
	Assert.Equals("key-2", session.SessionKeys["example.com"])
}

func (test *SessionServiceTest) TestRenewEndsASessionThatCannotRefresh() {
	session := test.signedIn()
	session.tokens.keys["example.com"] = "ended"
	session.Renew()
	Assert.Equals("expired", session.Step)
}

func (test *SessionServiceTest) TestForgetLabelNamesTheDomain() {
	Assert.Equals("Remove example.org from this device", test.started().ForgetLabel("example.org"))
}

func (test *SessionServiceTest) TestSignInWithTakesTheFormValues() {
	session := test.started()
	session.SignInWith("ana@example.com", "right", "", true)
	Assert.Equals("signedIn", session.Step)
	Assert.Equals(1, len(session.RecentDomains))
}

func (test *SessionServiceTest) TestShowPasswordWithKeepsWhatWasTyped() {
	session := test.started()
	session.ShowPasswordWith(true, "ana@example.com", "secret")
	Assert.IsTrue(session.ShowPassword)
	Assert.Equals("ana@example.com", session.ShownEmail)
	Assert.Equals("secret", session.ShownPassword)
}

func (test *SessionServiceTest) TestSignInForgetsTheShownPassword() {
	session := test.started()
	session.ShowPasswordWith(true, "ana@example.com", "right")
	session.SignInWith("ana@example.com", "right", "", false)
	Assert.Equals("", session.ShownPassword)
}
