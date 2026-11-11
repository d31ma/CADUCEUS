import "errors"

// @Test(SessionTestSetup, {"partition":"tests"})
type SessionProblemServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionProblemServiceTest) status(status int) error {
	return &BehaviorError{Code: "HTTP_STATUS", Status: status}
}

func (test *SessionProblemServiceTest) TestOfSignInIsEmptyWithoutFailure() {
	Assert.Equals("", (&SessionProblemService{}).OfSignIn(nil, "ana"))
}

func (test *SessionProblemServiceTest) TestOfSignInNamesWrongCredentials() {
	Assert.Equals("wrongCredentials", (&SessionProblemService{}).OfSignIn(test.status(401), "ana"))
}

func (test *SessionProblemServiceTest) TestOfSignInWithAnAddressNamesNotAMember() {
	Assert.Equals("notAMember", (&SessionProblemService{}).OfSignIn(test.status(403), "ana@partner.test"))
}

func (test *SessionProblemServiceTest) TestOfSignInWithAUsernameNamesNoDomain() {
	Assert.Equals("noDomain", (&SessionProblemService{}).OfSignIn(test.status(403), "dan"))
}

func (test *SessionProblemServiceTest) TestOfSignInNamesTooManyTries() {
	Assert.Equals("rateLimited", (&SessionProblemService{}).OfSignIn(test.status(429), "ana"))
}

func (test *SessionProblemServiceTest) TestOfSignInCallsOtherStatusesUnavailable() {
	Assert.Equals("unavailable", (&SessionProblemService{}).OfSignIn(test.status(503), "ana"))
}

func (test *SessionProblemServiceTest) TestOfSignInCallsPlainErrorsUnavailable() {
	Assert.Equals("unavailable", (&SessionProblemService{}).OfSignIn(errors.New("offline"), "ana"))
}

func (test *SessionProblemServiceTest) TestOfLoginAcceptsAUsername() {
	Assert.Equals("", (&SessionProblemService{}).OfLogin(" ana "))
}

func (test *SessionProblemServiceTest) TestOfLoginAcceptsAnAddress() {
	Assert.Equals("", (&SessionProblemService{}).OfLogin("ana@example.org"))
}

func (test *SessionProblemServiceTest) TestOfLoginRefusesAnEmptyField() {
	Assert.Equals("invalidLogin", (&SessionProblemService{}).OfLogin("   "))
}

func (test *SessionProblemServiceTest) TestOfLoginRefusesATrailingAt() {
	Assert.Equals("invalidLogin", (&SessionProblemService{}).OfLogin("ana@"))
}

func (test *SessionProblemServiceTest) TestOfSwitchNamesARemovedDomain() {
	Assert.Equals("notMember", (&SessionProblemService{}).OfSwitch(test.status(404)))
}

func (test *SessionProblemServiceTest) TestOfSwitchNamesAnEndedSession() {
	Assert.Equals("ended", (&SessionProblemService{}).OfSwitch(test.status(401)))
	Assert.Equals("ended", (&SessionProblemService{}).OfSwitch(test.status(403)))
}

func (test *SessionProblemServiceTest) TestOfSwitchCallsOtherFailuresUnavailable() {
	Assert.Equals("unavailable", (&SessionProblemService{}).OfSwitch(errors.New("busy")))
}

func (test *SessionProblemServiceTest) TestOfDefaultDomainNamesARemovedDomain() {
	Assert.Equals("notMember", (&SessionProblemService{}).OfDefaultDomain(test.status(404)))
	Assert.Equals("", (&SessionProblemService{}).OfDefaultDomain(nil))
	Assert.Equals("unavailable", (&SessionProblemService{}).OfDefaultDomain(test.status(500)))
}

func (test *SessionProblemServiceTest) TestOfAccountChangeIsEmptyWithoutFailure() {
	Assert.Equals("", (&SessionProblemService{}).OfAccountChange(nil, "passwordPolicy"))
}

func (test *SessionProblemServiceTest) TestOfAccountChangeNamesAWrongPassword() {
	Assert.Equals("wrongPassword", (&SessionProblemService{}).OfAccountChange(test.status(401), "passwordPolicy"))
}

func (test *SessionProblemServiceTest) TestOfAccountChangeNamesTheRefusal() {
	Assert.Equals("passwordPolicy", (&SessionProblemService{}).OfAccountChange(test.status(400), "passwordPolicy"))
}

func (test *SessionProblemServiceTest) TestOfAccountChangeNamesTooManyTries() {
	Assert.Equals("rateLimited", (&SessionProblemService{}).OfAccountChange(test.status(429), "invalidCode"))
}
