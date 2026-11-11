// @Test(AdminSessionTestSetup, {"partition":"tests"})
type AdminSessionServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	identityClient *IdentityClient
}

func (test *AdminSessionServiceTest) started() *AdminSessionService {
	session := &AdminSessionService{}
	test.construct(session)
	return session
}

func (test *AdminSessionServiceTest) TestStartShowsTheSignInForm() {
	session := test.started()
	session.Start()
	Assert.Equals("signedOut", session.Step)
	Assert.Equals("Your username, like ana, or your full address.", session.Hint)
}

func (test *AdminSessionServiceTest) TestAnOperatorSignsInWithTheUsernameOnly() {
	session := test.started()
	session.SignInWith("olu", "right", "")
	Assert.Equals("ready", session.Step)
	Assert.Equals(true, session.Operator)
	Assert.Equals(false, session.DomainAdmin)
	Assert.Equals("*", session.DomainId)
	Assert.Equals("operator-key", session.SessionKey)
}

func (test *AdminSessionServiceTest) TestTheLoginIsTrimmedAndLowered() {
	session := test.started()
	session.SignInWith("  OLU ", "right", "")
	Assert.Equals("ready", session.Step)
	Assert.Equals(true, session.Operator)
}

func (test *AdminSessionServiceTest) TestADomainAdminSignsInWithTheFullAddress() {
	session := test.started()
	session.SignInWith("ana@example.com", "right", "")
	Assert.Equals("ready", session.Step)
	Assert.Equals("example.com", session.DomainId)
	Assert.Equals("Ex", session.Initials)
	Assert.Equals(true, session.DomainAdmin)
	Assert.Equals("ana", session.Username)
}

func (test *AdminSessionServiceTest) TestTilesShowOnlyTheAdministeredDomains() {
	session := test.started()
	session.SignInWith("ana@example.com", "right", "")
	Assert.Equals(1, len(session.Tiles))
	Assert.Equals("example.com", session.Tiles[0].DomainId)
	Assert.Equals(true, session.Tiles[0].Current)
	Assert.Equals(2, len(session.Domains))
}

func (test *AdminSessionServiceTest) TestAnAdminOfAnotherDomainIsMovedThere() {
	session := test.started()
	session.SignInWith("carl", "right", "")
	Assert.Equals("ready", session.Step)
	Assert.Equals("example.org", session.DomainId)
	Assert.Equals(true, session.DomainAdmin)
	Assert.Equals("Bearer switched-token", session.Authorization())
}

func (test *AdminSessionServiceTest) TestSignInKeepsOnlyTheLogin() {
	session := test.started()
	session.SignInWith("ana@example.com", "right", "123456")
	Assert.Equals("ana@example.com", session.Login)
	Assert.Equals(false, session.Busy)
}

func (test *AdminSessionServiceTest) TestAnEmptyLoginIsCheckedBeforeSending() {
	session := test.started()
	session.Start()
	session.SignInWith("  ", "right", "")
	Assert.Equals("loginMissing", session.Problem)
	Assert.Equals("signedOut", session.Step)
}

func (test *AdminSessionServiceTest) TestALoginThatEndsInAtIsCheckedBeforeSending() {
	session := test.started()
	session.SignInWith("ana@", "right", "")
	Assert.Equals("loginMissing", session.Problem)
}

func (test *AdminSessionServiceTest) TestSignInRefusesAMemberWithoutAdminRights() {
	session := test.started()
	session.SignInWith("bo@example.com", "right", "")
	Assert.Equals("noRights", session.Step)
	Assert.Equals("Bo Lee", session.DisplayName)
}

func (test *AdminSessionServiceTest) TestAWrongPasswordStaysSignedOut() {
	session := test.started()
	session.SignInWith("ana@example.com", "wrong", "")
	Assert.Equals("signedOut", session.Step)
	Assert.Equals("refused", session.Problem)
	Assert.Equals("", session.SessionKey)
}

func (test *AdminSessionServiceTest) TestAnAddressOfAnotherDomainSaysNotAMember() {
	session := test.started()
	session.SignInWith("ana@partner.test", "right", "")
	Assert.Equals("notMember", session.Problem)
	Assert.Equals("signedOut", session.Step)
}

func (test *AdminSessionServiceTest) TestAUsernameWithoutDomainsSaysNoDomain() {
	session := test.started()
	session.SignInWith("finn", "right", "")
	Assert.Equals("noDomain", session.Problem)
}

func (test *AdminSessionServiceTest) TestUseUsernameOnlyKeepsThePrefix() {
	session := test.started()
	session.SignInWith("ana@partner.test", "right", "")
	session.UseUsernameOnly()
	Assert.Equals("ana", session.Login)
	Assert.Equals("retype", session.Problem)
	Assert.Equals("Opens your default domain, or the whole install for an Operator.", session.Hint)
}

func (test *AdminSessionServiceTest) TestChangeAddressClosesTheNotice() {
	session := test.started()
	session.SignInWith("ana@partner.test", "right", "")
	session.ChangeAddress()
	Assert.Equals("", session.Problem)
}

func (test *AdminSessionServiceTest) TestTypeLoginExplainsAFullAddress() {
	session := test.started()
	session.TypeLogin("Ana@Example.ORG")
	Assert.Equals("Opens example.org.", session.Hint)
}

func (test *AdminSessionServiceTest) TestALockedAccountIsRateLimited() {
	session := test.started()
	session.SignInWith("busy", "right", "")
	Assert.Equals("rateLimited", session.Problem)
}

func (test *AdminSessionServiceTest) TestAuthorizationIsABearerHeader() {
	session := test.started()
	session.SignInWith("ana@example.com", "right", "")
	Assert.Equals("Bearer admin-token", session.Authorization())
}

func (test *AdminSessionServiceTest) TestSwitchToOpensAnotherDomain() {
	session := test.started()
	session.SignInWith("ana@example.com", "right", "")
	session.SwitchTo("example.org")
	Assert.Equals("example.org", session.DomainId)
	Assert.Equals("", session.Switching)
	Assert.Equals("", session.SwitchProblem)
}

func (test *AdminSessionServiceTest) TestSwitchToARemovedDomainKeepsTheOpenOne() {
	session := test.started()
	session.SignInWith("ana@example.com", "right", "")
	session.SwitchTo("gone.test")
	Assert.Equals("notMember", session.SwitchProblem)
	Assert.Equals("example.com", session.DomainId)
	Assert.Equals("Bearer admin-token", session.Authorization())
}

func (test *AdminSessionServiceTest) TestResumeRenewsASavedSession() {
	session := &AdminSessionService{SessionKey: "admin-key"}
	test.construct(session)
	session.Resume(AdminPageStarted{Started: true})
	Assert.Equals("ready", session.Step)
	Assert.Equals("Bearer admin-token-2", session.Authorization())
}

func (test *AdminSessionServiceTest) TestResumeWithAnEndedSessionAsksToSignInAgain() {
	session := &AdminSessionService{SessionKey: "ended-key"}
	test.construct(session)
	session.Resume(AdminPageStarted{Started: true})
	Assert.Equals("signedOut", session.Step)
	Assert.Equals("expired", session.Problem)
}

func (test *AdminSessionServiceTest) TestResumeWithoutASavedSessionWaits() {
	session := test.started()
	session.Resume(AdminPageStarted{Started: true})
	Assert.Equals("", session.Problem)
}

func (test *AdminSessionServiceTest) TestSignOutForgetsTheSession() {
	session := test.started()
	session.SignInWith("ana@example.com", "right", "")
	session.SignOut()
	Assert.Equals("signedOut", session.Step)
	Assert.Equals("", session.SessionKey)
	Assert.Equals(0, len(session.Tiles))
}
