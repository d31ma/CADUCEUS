// @Test(AdminSessionTestSetup, {"partition":"tests"})
type AdminSessionServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	identityClient *IdentityClient
}

func (test *AdminSessionServiceTest) TestStartShowsTheSignInForm() {
	session := &AdminSessionService{}
	test.construct(session)
	session.Start()
	Assert.Equals("signedOut", session.Step)
}

func (test *AdminSessionServiceTest) TestSignInSignsInAnOperator() {
	session := &AdminSessionService{}
	test.construct(session)
	session.SignInWith("olu@mesail-operators", "right", "")
	Assert.Equals("ready", session.Step)
	Assert.Equals(true, session.Operator)
	Assert.Equals(false, session.DomainAdmin)
	Assert.Equals("operator-key", session.SessionKey)
}

func (test *AdminSessionServiceTest) TestSignInSignsInADomainAdminOfTheirDomain() {
	session := &AdminSessionService{}
	test.construct(session)
	session.SignInWith("ana@example.com", "right", "")
	Assert.Equals("ready", session.Step)
	Assert.Equals("example.com", session.DomainId)
	Assert.Equals("Ex", session.Initials)
	Assert.Equals(true, session.DomainAdmin)
}

func (test *AdminSessionServiceTest) TestSignInKeepsOnlyTheAddress() {
	session := &AdminSessionService{}
	test.construct(session)
	session.SignInWith("ana@example.com", "right", "123456")
	Assert.Equals("ana@example.com", session.Address)
	Assert.Equals(false, session.Busy)
}

func (test *AdminSessionServiceTest) TestSignInRefusesAMemberWithoutAdminRights() {
	session := &AdminSessionService{}
	test.construct(session)
	session.SignInWith("bo@example.com", "right", "")
	Assert.Equals("noRights", session.Step)
}

func (test *AdminSessionServiceTest) TestAWrongPasswordStaysSignedOut() {
	session := &AdminSessionService{}
	test.construct(session)
	session.SignInWith("ana@example.com", "wrong", "")
	Assert.Equals("signedOut", session.Step)
	Assert.Equals("refused", session.Problem)
	Assert.Equals("", session.SessionKey)
}

func (test *AdminSessionServiceTest) TestALockedAccountIsRateLimited() {
	session := &AdminSessionService{}
	test.construct(session)
	session.SignInWith("busy@example.com", "right", "")
	Assert.Equals("rateLimited", session.Problem)
}

func (test *AdminSessionServiceTest) TestAuthorizationIsABearerHeader() {
	session := &AdminSessionService{}
	test.construct(session)
	session.SignInWith("ana@example.com", "right", "")
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
	session := &AdminSessionService{}
	test.construct(session)
	session.Resume(AdminPageStarted{Started: true})
	Assert.Equals("", session.Problem)
}

func (test *AdminSessionServiceTest) TestSignOutForgetsTheSession() {
	session := &AdminSessionService{}
	test.construct(session)
	session.SignInWith("ana@example.com", "right", "")
	session.SignOut()
	Assert.Equals("signedOut", session.Step)
	Assert.Equals("", session.SessionKey)
}
