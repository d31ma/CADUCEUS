import "strings"

// @Test(SessionTestSetup, {"partition":"tests"})
type SessionServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionServiceTest) session() *SessionService {
	BrowserFakes{}.Answer("secureRandom", "bytes", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32})
	BrowserFakes{}.Answer("clock", "now", 1000)
	client := NewIdentityClientWith(test.behaviors)
	session := &SessionService{
		proof:    &SessionProofService{},
		realm:    &SessionRealmService{identity: client},
		tokens:   &SessionTokenService{identity: client},
		accounts: &SessionAccountService{identity: client},
		problems: &SessionProblemService{},
	}
	session.Start()
	return session
}

func (test *SessionServiceTest) found() *SessionService {
	session := test.session()
	session.Email = "ana@example.com"
	session.Continue()
	return session
}

func (test *SessionServiceTest) signedIn() *SessionService {
	session := test.found()
	session.Leave()
	session.Finish("good-code", session.Pending.State)
	return session
}

func (test *SessionServiceTest) TestStartAsksForTheEmail() {
	session := test.session()
	Assert.Equals("email", session.Step)
	Assert.Equals("", session.Problem)
	Assert.IsTrue(session.Remember)
}

func (test *SessionServiceTest) TestContinueRefusesAnIncompleteEmail() {
	session := test.session()
	session.Email = "ana"
	session.Continue()
	Assert.Equals("email", session.Step)
	Assert.Equals("invalidEmail", session.Problem)
	Assert.Equals("", session.AuthorizeUrl)
}

func (test *SessionServiceTest) TestContinueFindsTheDomainRealm() {
	session := test.found()
	Assert.Equals("domain", session.Step)
	Assert.Equals("example.com", session.DomainId)
	Assert.Equals("sign-in.example.test", session.SignInHost)
	Assert.IsTrue(strings.Contains(session.AuthorizeUrl, "/realms/example.com/"), session.AuthorizeUrl)
}

func (test *SessionServiceTest) TestContinueKeepsTheProofForTheReturn() {
	session := test.found()
	Assert.Equals("example.com", session.Pending.DomainId)
	Assert.Equals(43, len(session.Pending.Verifier))
	Assert.IsTrue(strings.Contains(session.AuthorizeUrl, "state="+session.Pending.State), session.AuthorizeUrl)
}

func (test *SessionServiceTest) TestContinueAsUsesARecentDomain() {
	session := test.session()
	session.ContinueAs(SessionRecentDomain{DomainId: "example.org", Email: "ana@example.org"})
	Assert.Equals("example.org", session.DomainId)
	Assert.Equals("ana@example.org", session.Email)
}

func (test *SessionServiceTest) TestLeaveRemembersTheDomain() {
	session := test.found()
	session.Leave()
	Assert.Equals("redirecting", session.Step)
	Assert.Equals([]SessionRecentDomain{{DomainId: "example.com", Email: "ana@example.com"}}, session.RecentDomains)
}

func (test *SessionServiceTest) TestLeaveWithoutRememberKeepsNothing() {
	session := test.found()
	session.Remember = false
	session.Leave()
	Assert.Equals(0, len(session.RecentDomains))
}

func (test *SessionServiceTest) TestLeavePutsTheDomainFirstOnce() {
	session := test.found()
	session.RecentDomains = []SessionRecentDomain{{DomainId: "example.org", Email: "ana@example.org"}, {DomainId: "example.com", Email: "old@example.com"}}
	session.Leave()
	Assert.Equals([]SessionRecentDomain{{DomainId: "example.com", Email: "ana@example.com"}, {DomainId: "example.org", Email: "ana@example.org"}}, session.RecentDomains)
}

func (test *SessionServiceTest) TestForgetRemovesOneRecentDomain() {
	session := test.found()
	session.Leave()
	session.Forget("example.com")
	Assert.Equals(0, len(session.RecentDomains))
}

func (test *SessionServiceTest) TestForgetAllRemovesEveryRecentDomain() {
	session := test.found()
	session.Leave()
	session.ForgetAll()
	Assert.Equals(0, len(session.RecentDomains))
}

func (test *SessionServiceTest) TestUseDifferentEmailGoesBack() {
	session := test.found()
	session.UseDifferentEmail()
	Assert.Equals("email", session.Step)
}

func (test *SessionServiceTest) TestCancelReturnsToTheDomain() {
	session := test.found()
	session.Leave()
	session.Cancel()
	Assert.Equals("domain", session.Step)
}

func (test *SessionServiceTest) TestFinishSignsIn() {
	session := test.signedIn()
	Assert.Equals("signedIn", session.Step)
	Assert.Equals("", session.Problem)
	Assert.Equals("example.com", session.accounts.CurrentDomain)
	Assert.Equals("", session.Pending.Verifier)
	Assert.IsTrue(strings.Contains(session.SignOutUrl, "id_token_hint=id-1"), session.SignOutUrl)
}

func (test *SessionServiceTest) TestFinishRefusesAnotherState() {
	session := test.found()
	session.Finish("good-code", "forged-state")
	Assert.Equals("problem", session.Step)
	Assert.Equals("startAgain", session.Problem)
	Assert.Equals(0, len(session.accounts.Accounts))
}

func (test *SessionServiceTest) TestFinishReportsARefusedCode() {
	session := test.found()
	session.Finish("used-code", session.Pending.State)
	Assert.Equals("idpDown", session.Problem)
}

func (test *SessionServiceTest) TestFinishReportsAnAccountThatCannotLoad() {
	session := test.found()
	session.Finish("disabled-code", session.Pending.State)
	Assert.Equals("idpDown", session.Problem)
	Assert.Equals(0, len(session.accounts.Accounts))
}

func (test *SessionServiceTest) TestFailNamesACancelledSignIn() {
	session := test.found()
	session.Fail("access_denied")
	Assert.Equals("problem", session.Step)
	Assert.Equals("cancelled", session.Problem)
}

func (test *SessionServiceTest) TestSwitchChangesTheSignOutLink() {
	session := test.signedIn()
	session.Switch("example.org")
	Assert.Equals("example.org", session.accounts.CurrentDomain)
	Assert.IsTrue(strings.Contains(session.SignOutUrl, "/realms/example.org/"), session.SignOutUrl)
}

func (test *SessionServiceTest) TestSignOutForgetsTheTokens() {
	session := test.signedIn()
	session.SignOut()
	Assert.Equals("signedOut", session.Step)
	Assert.Equals("", session.tokens.IdToken("example.com"))
}

func (test *SessionServiceTest) TestShownKeepsAWorkingSession() {
	session := test.signedIn()
	session.Shown(VisibilityChanged{Visible: true})
	Assert.Equals("signedIn", session.Step)
}

func (test *SessionServiceTest) TestShownEndsASessionThatCannotRefresh() {
	session := test.signedIn()
	session.tokens.Forget("example.com")
	session.Shown(VisibilityChanged{Visible: true})
	Assert.Equals("expired", session.Step)
}

func (test *SessionServiceTest) TestShownIgnoresAHiddenPage() {
	session := test.signedIn()
	session.tokens.Forget("example.com")
	session.Shown(VisibilityChanged{Visible: false})
	Assert.Equals("signedIn", session.Step)
}
