import "errors"

// @Test(KeycloakTestSetup, {"partition":"tests"})
type AccountSessionTest struct {
	behaviors *BehaviorContext
}

func (test *AccountSessionTest) signIn(password string) (Session, error) {
	setup := KeycloakTestSetup{}
	setup.Keycloak(setup.Realm("example.com"))
	return (&AccountController{}).PostSession(SignInRequest{Address: "ana@example.com", Password: password, OneTimeCode: "123456"})
}

func (test *AccountSessionTest) TestPostSessionAnswersTheSession() {
	session, err := test.signIn("right")
	Assert.IsTrue(err == nil)
	Assert.Equals("example.com", session.Me.DomainId)
	Assert.Equals("member-1", session.Me.MemberId)
	Assert.Equals(int64(60), session.ExpiresIn)
	Assert.IsTrue(session.AccessToken != "")
	Assert.IsTrue(session.SessionKey != "")
}

func (test *AccountSessionTest) TestPostSessionRefusesAWrongPassword() {
	_, err := test.signIn("wrong")
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
}

func (test *AccountSessionTest) TestPostSessionReportsALockedAccount() {
	_, err := test.signIn("locked")
	var limited *RateLimited
	Assert.IsTrue(errors.As(err, &limited))
}

func (test *AccountSessionTest) TestPostSessionIsUnavailableWithoutSettings() {
	KeycloakTestSetup{}.Unconfigured()
	_, err := (&AccountController{}).PostSession(SignInRequest{Address: "ana@example.com", Password: "right"})
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable))
}

func (test *AccountSessionTest) TestPostRefreshAnswersANewSession() {
	session, _ := test.signIn("right")
	refreshed, err := (&AccountController{}).PostRefresh(RefreshRequest{SessionKey: session.SessionKey})
	Assert.IsTrue(err == nil)
	Assert.Equals("member-1", refreshed.Me.MemberId)
	Assert.IsTrue(refreshed.SessionKey != session.SessionKey)
}

func (test *AccountSessionTest) TestPostRefreshRefusesAForgedSessionKey() {
	test.signIn("right")
	_, err := (&AccountController{}).PostRefresh(RefreshRequest{SessionKey: "forged"})
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
}

func (test *AccountSessionTest) TestDeleteSessionEndsTheSession() {
	session, _ := test.signIn("right")
	Assert.IsTrue((&AccountController{}).DeleteSession(RefreshRequest{SessionKey: session.SessionKey}) == nil)
}
