// @Test(SessionTestSetup, {"partition":"tests"})
type SessionServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionServiceTest) build(key string) *SessionService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	client := NewIdentityClientWith(test.behaviors)
	return &SessionService{
		tokens:     &SessionTokenService{identity: client},
		accounts:   &SessionAccountService{},
		problems:   &SessionProblemService{},
		SessionKey: key,
	}
}

func (test *SessionServiceTest) started() *SessionService {
	session := test.build("")
	session.Start()
	session.Restore(SessionRestoreRequested{})
	return session
}

func (test *SessionServiceTest) signInAs(session *SessionService, login string, password string, remember bool) *SessionService {
	session.SignInWith(login, password, "", remember)
	session.SignIn(SessionSignInRequested{})
	return session
}

func (test *SessionServiceTest) signedIn() *SessionService {
	return test.signInAs(test.started(), "ana", "right", true)
}

func (test *SessionServiceTest) TestStartWaitsForTheRestore() {
	session := test.build("")
	session.Start()
	Assert.Equals("", session.Step)
}

func (test *SessionServiceTest) TestStartWithoutSessionShowsTheForm() {
	Assert.Equals("signIn", test.started().Step)
}

func (test *SessionServiceTest) TestStartRestoresTheTabSession() {
	session := test.build("key-1")
	session.Restore(SessionRestoreRequested{})
	Assert.Equals("signedIn", session.Step)
	Assert.Equals("example.com", session.accounts.CurrentDomain)
	Assert.Equals("key-2", session.SessionKey)
}

func (test *SessionServiceTest) TestStartWithAnEndedSessionShowsTheForm() {
	session := test.build("ended")
	session.Restore(SessionRestoreRequested{})
	Assert.Equals("signIn", session.Step)
	Assert.Equals("", session.SessionKey)
}

func (test *SessionServiceTest) TestStartFillsTheRememberedUsername() {
	session := test.build("")
	session.RememberedUsername = "ana"
	session.Start()
	Assert.Equals("ana", session.Login)
	Assert.IsTrue(session.Remember)
}

func (test *SessionServiceTest) TestSignInWithAUsernameOpensTheDefaultDomain() {
	session := test.signedIn()
	Assert.Equals("signedIn", session.Step)
	Assert.Equals("example.com", session.accounts.CurrentDomain)
	Assert.Equals("key-1", session.SessionKey)
	Assert.Equals(2, len(session.accounts.Accounts))
}

func (test *SessionServiceTest) TestSignInWithAnAddressOpensThatDomain() {
	session := test.signInAs(test.started(), "ana@example.org", "right", false)
	Assert.Equals("example.org", session.accounts.CurrentDomain)
}

func (test *SessionServiceTest) TestSignInStartsInTheBackground() {
	session := test.started()
	session.SignInWith("ana", "right", "", false)
	Assert.IsTrue(session.Busy)
	Assert.Equals("signIn", session.Step)
}

func (test *SessionServiceTest) TestSignInClearsThePasswordAndCode() {
	session := test.started()
	session.SignInWith("ana", "right", "123456", false)
	session.SignIn(SessionSignInRequested{})
	Assert.Equals("", session.Password)
	Assert.Equals("", session.OneTimeCode)
	Assert.Equals(false, session.Busy)
}

func (test *SessionServiceTest) TestSignInRefusesAnEmptyField() {
	session := test.started()
	session.SignInWith("  ", "right", "", false)
	Assert.Equals("invalidLogin", session.Problem)
	Assert.Equals(false, session.Busy)
}

func (test *SessionServiceTest) TestSignInRefusesATrailingAt() {
	session := test.started()
	session.SignInWith("ana@", "right", "", false)
	Assert.Equals("invalidLogin", session.Problem)
}

func (test *SessionServiceTest) TestSignInRemembersOnlyTheUsername() {
	session := test.signInAs(test.started(), "ana@example.org", "right", true)
	Assert.Equals("ana", session.RememberedUsername)
}

func (test *SessionServiceTest) TestSignInWithoutRememberForgetsTheUsername() {
	session := test.started()
	session.RememberedUsername = "ana"
	test.signInAs(session, "ana", "right", false)
	Assert.Equals("", session.RememberedUsername)
}

func (test *SessionServiceTest) TestSignInRefusedShowsOneProblem() {
	session := test.signInAs(test.started(), "ana", "wrong", false)
	Assert.Equals("wrongCredentials", session.Problem)
	Assert.Equals("signIn", session.Step)
	Assert.Equals("", session.Password)
}

func (test *SessionServiceTest) TestSignInToAnotherDomainKeepsThePasswordForTheRetry() {
	session := test.signInAs(test.started(), "ana@partner.test", "right", false)
	Assert.Equals("notAMember", session.Problem)
	Assert.Equals("partner.test", session.ProblemDomain)
	Assert.Equals("right", session.Password)
}

func (test *SessionServiceTest) TestSignInAsUsernameRetriesWithThePrefixOnly() {
	session := test.signInAs(test.started(), "ana@partner.test", "right", false)
	session.SignInAsUsername()
	Assert.Equals("ana", session.Login)
	session.SignIn(SessionSignInRequested{})
	Assert.Equals("signedIn", session.Step)
	Assert.Equals("example.com", session.accounts.CurrentDomain)
}

func (test *SessionServiceTest) TestChangeAddressKeepsThePasswordInTheField() {
	session := test.signInAs(test.started(), "ana@partner.test", "right", false)
	session.ChangeAddress()
	Assert.Equals("", session.Problem)
	Assert.Equals("right", session.ShownPassword)
	Assert.Equals("", session.Password)
}

func (test *SessionServiceTest) TestSignInWithoutADomainShowsTheNoDomainCard() {
	session := test.signInAs(test.started(), "dan", "right", false)
	Assert.Equals("noDomain", session.Problem)
	Assert.Equals("noDomain", session.Step)
	session.BackToSignIn()
	Assert.Equals("signIn", session.Step)
}

func (test *SessionServiceTest) TestSignInWithTooManyTriesSaysSo() {
	Assert.Equals("rateLimited", test.signInAs(test.started(), "ana", "busy", false).Problem)
}

func (test *SessionServiceTest) TestSignInThatMustChangeThePasswordShowsOnlyThatScreen() {
	Assert.Equals("choosePassword", test.signInAs(test.started(), "new", "right", false).Step)
}

func (test *SessionServiceTest) TestSignInOfAnOperatorWithoutDomainShowsTheAdminLink() {
	Assert.Equals("noDomainOperator", test.signInAs(test.started(), "op", "right", false).Step)
}

func (test *SessionServiceTest) TestUsernameOfTakesThePartBeforeTheAt() {
	Assert.Equals("ana", test.started().UsernameOf(" ANA@example.org"))
}

func (test *SessionServiceTest) TestUseAnotherUsernameForgetsTheRememberedOne() {
	session := test.signedIn()
	session.UseAnotherUsername()
	Assert.Equals("", session.RememberedUsername)
	Assert.Equals("", session.Login)
}

func (test *SessionServiceTest) TestSwitchStartsInTheBackground() {
	session := test.signedIn()
	session.Switch("example.org")
	Assert.Equals("example.org", session.Switching)
	Assert.Equals("example.com", session.accounts.CurrentDomain)
}

func (test *SessionServiceTest) TestSwitchToTheCurrentDomainDoesNothing() {
	session := test.signedIn()
	session.Switch("example.com")
	Assert.Equals("", session.Switching)
}

func (test *SessionServiceTest) TestSwitchToOpensTheDomain() {
	session := test.signedIn()
	session.Switch("example.org")
	session.SwitchTo(SessionSwitchRequested{DomainId: "example.org"})
	Assert.Equals("example.org", session.accounts.CurrentDomain)
	Assert.Equals("key-org", session.SessionKey)
	Assert.Equals("Now in example.org.", session.Notice)
	Assert.Equals("", session.Switching)
}

func (test *SessionServiceTest) TestSwitchToARemovedDomainDropsItFromTheList() {
	session := test.signedIn()
	session.accounts.Accounts = append(session.accounts.Accounts, SessionAccount{DomainId: "gone.example"})
	session.SwitchTo(SessionSwitchRequested{DomainId: "gone.example"})
	Assert.Equals("example.com", session.accounts.CurrentDomain)
	Assert.Equals(2, len(session.accounts.Accounts))
	Assert.Equals("You are no longer a member of gone.example, so it is gone from your list.", session.Notice)
}

func (test *SessionServiceTest) TestSwitchToWithAnEndedSessionShowsTheEndedDialog() {
	session := test.signedIn()
	session.tokens.key = "ended"
	session.SwitchTo(SessionSwitchRequested{DomainId: "example.org"})
	Assert.Equals("expired", session.Step)
}

func (test *SessionServiceTest) TestAccessTokenFollowsAMoveToTheDefaultDomain() {
	session := test.signInAs(test.started(), "ana@example.org", "right", false)
	session.tokens.key = "key-removed"
	BrowserFakes{}.Answer("clock", "now", 50000)
	accessToken, failure := session.AccessToken()
	Assert.Equals(nil, failure)
	Assert.Equals("access-back", accessToken)
	Assert.Equals("example.com", session.accounts.CurrentDomain)
	Assert.Equals("You no longer have access to example.org. Now in example.com.", session.Notice)
}

func (test *SessionServiceTest) TestAccessTokenGivesTheTabToken() {
	session := test.signedIn()
	accessToken, _ := session.AccessToken()
	Assert.Equals("access-1", accessToken)
	Assert.Equals("", session.Notice)
}

func (test *SessionServiceTest) TestRenewGivesAFreshTokenAndSavesTheKey() {
	session := test.signedIn()
	accessToken, failure := session.Renew()
	Assert.Equals(nil, failure)
	Assert.Equals("access-2", accessToken)
	Assert.Equals("key-2", session.SessionKey)
}

func (test *SessionServiceTest) TestRenewEndsASessionThatCannotRefresh() {
	session := test.signedIn()
	session.tokens.key = "ended"
	session.Renew()
	Assert.Equals("expired", session.Step)
}

func (test *SessionServiceTest) TestChangePasswordSavesTheNewKey() {
	session := test.signedIn()
	Assert.Equals(nil, session.ChangePassword(SessionPasswordChange{CurrentPassword: "right", NewPassword: "a-long-new-password"}))
	Assert.Equals("key-new", session.SessionKey)
}

func (test *SessionServiceTest) TestSignOutEndsTheSessionInEveryDomain() {
	session := test.signedIn()
	session.SignOut()
	Assert.Equals("signedOut", session.Step)
	Assert.Equals("", session.SessionKey)
	Assert.Equals(0, len(session.accounts.Accounts))
}

func (test *SessionServiceTest) TestSignInAgainFillsTheRememberedUsername() {
	session := test.signedIn()
	session.SignOut()
	session.SignInAgain()
	Assert.Equals("signIn", session.Step)
	Assert.Equals("ana", session.Login)
}

func (test *SessionServiceTest) TestForgetUsernameClearsIt() {
	session := test.signedIn()
	session.ForgetUsername()
	Assert.Equals("", session.RememberedUsername)
}

func (test *SessionServiceTest) TestShownKeepsAWorkingSession() {
	session := test.signedIn()
	session.Shown(VisibilityChanged{Visible: true})
	Assert.Equals("signedIn", session.Step)
}

func (test *SessionServiceTest) TestShownEndsASessionThatCannotRefresh() {
	session := test.signedIn()
	session.tokens.key = "ended"
	BrowserFakes{}.Answer("clock", "now", 50000)
	session.Shown(VisibilityChanged{Visible: true})
	Assert.Equals("expired", session.Step)
}

func (test *SessionServiceTest) TestShownIgnoresAHiddenPage() {
	session := test.signedIn()
	session.tokens.key = "ended"
	BrowserFakes{}.Answer("clock", "now", 50000)
	session.Shown(VisibilityChanged{Visible: false})
	Assert.Equals("signedIn", session.Step)
}

func (test *SessionServiceTest) TestShowPasswordWithKeepsWhatWasTyped() {
	session := test.started()
	session.ShowPasswordWith(true, "ana", "secret")
	Assert.IsTrue(session.ShowPassword)
	Assert.Equals("ana", session.Login)
	Assert.Equals("secret", session.ShownPassword)
}

func (test *SessionServiceTest) TestDismissNoticeClearsIt() {
	session := test.signedIn()
	session.Notice = "Now in example.org."
	session.DismissNotice()
	Assert.Equals("", session.Notice)
}

func (test *SessionServiceTest) TestPasswordChosenOpensTheDomain() {
	session := test.signInAs(test.started(), "new", "right", false)
	session.tokens.ChangePassword(SessionPasswordChange{CurrentPassword: "right", NewPassword: "a-long-new-password"})
	session.PasswordChosen()
	Assert.Equals("signedIn", session.Step)
}

func (test *SessionServiceTest) TestRenewAfterSignOutKeepsTheSignedOutPage() {
	session := test.signedIn()
	session.SignOut()
	session.Renew()
	Assert.Equals("signedOut", session.Step)
}
