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
	session := &AdminSessionService{Address: "olu@mesail-operators", Password: "right"}
	test.construct(session)
	session.SignIn()
	Assert.Equals("ready", session.Step)
	Assert.Equals(true, session.Operator)
	Assert.Equals(false, session.DomainAdmin)
	Assert.Equals("operator-key", session.SessionKey)
}

func (test *AdminSessionServiceTest) TestSignInSignsInADomainAdminOfTheirDomain() {
	session := &AdminSessionService{Address: "ana@example.com", Password: "right"}
	test.construct(session)
	session.SignIn()
	Assert.Equals("ready", session.Step)
	Assert.Equals("example.com", session.DomainId)
	Assert.Equals("Ex", session.Initials)
	Assert.Equals(true, session.DomainAdmin)
}

func (test *AdminSessionServiceTest) TestSignInForgetsThePasswordAndCode() {
	session := &AdminSessionService{Address: "ana@example.com", Password: "right", OneTimeCode: "123456"}
	test.construct(session)
	session.SignIn()
	Assert.Equals("", session.Password)
	Assert.Equals("", session.OneTimeCode)
	Assert.Equals(false, session.Busy)
}

func (test *AdminSessionServiceTest) TestSignInRefusesAMemberWithoutAdminRights() {
	session := &AdminSessionService{Address: "bo@example.com", Password: "right"}
	test.construct(session)
	session.SignIn()
	Assert.Equals("noRights", session.Step)
}

func (test *AdminSessionServiceTest) TestAWrongPasswordStaysSignedOut() {
	session := &AdminSessionService{Address: "ana@example.com", Password: "wrong"}
	test.construct(session)
	session.SignIn()
	Assert.Equals("signedOut", session.Step)
	Assert.Equals("refused", session.Problem)
	Assert.Equals("", session.SessionKey)
}

func (test *AdminSessionServiceTest) TestALockedAccountIsRateLimited() {
	session := &AdminSessionService{Address: "busy@example.com", Password: "right"}
	test.construct(session)
	session.SignIn()
	Assert.Equals("rateLimited", session.Problem)
}

func (test *AdminSessionServiceTest) TestAuthorizationIsABearerHeader() {
	session := &AdminSessionService{Address: "ana@example.com", Password: "right"}
	test.construct(session)
	session.SignIn()
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
	session := &AdminSessionService{Address: "ana@example.com", Password: "right"}
	test.construct(session)
	session.SignIn()
	session.SignOut()
	Assert.Equals("signedOut", session.Step)
	Assert.Equals("", session.SessionKey)
}
