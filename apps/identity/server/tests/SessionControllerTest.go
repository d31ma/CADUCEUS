import "errors"

// @Test(KeycloakTestSetup, {"partition":"tests"})
type SessionControllerTest struct {
	behaviors *BehaviorContext
}

func (test *SessionControllerTest) signIn(password string) (Session, error) {
	setup := KeycloakTestSetup{}
	setup.Keycloak(setup.Realm("example.com"))
	return (&SessionController{}).PostSession(SignInRequest{Address: "ana@example.com", Password: password, OneTimeCode: "123456"})
}

func (test *SessionControllerTest) TestPostSessionAnswersTheSession() {
	session, err := test.signIn("right")
	Assert.IsTrue(err == nil)
	Assert.Equals("example.com", session.Me.DomainId)
	Assert.Equals("member-1", session.Me.MemberId)
	Assert.Equals(int64(60), session.ExpiresIn)
	Assert.IsTrue(session.AccessToken != "")
	Assert.IsTrue(session.SessionKey != "")
}

func (test *SessionControllerTest) TestPostSessionRefusesAWrongPassword() {
	_, err := test.signIn("wrong")
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
}

func (test *SessionControllerTest) TestPostSessionReportsALockedAccount() {
	_, err := test.signIn("locked")
	var limited *RateLimited
	Assert.IsTrue(errors.As(err, &limited))
}

func (test *SessionControllerTest) TestPostSessionIsUnavailableWithoutSettings() {
	KeycloakTestSetup{}.Unconfigured()
	_, err := (&SessionController{}).PostSession(SignInRequest{Address: "ana@example.com", Password: "right"})
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable))
}

func (test *SessionControllerTest) TestPostRefreshAnswersANewSession() {
	session, _ := test.signIn("right")
	refreshed, err := (&SessionController{}).PostRefresh(RefreshRequest{SessionKey: session.SessionKey})
	Assert.IsTrue(err == nil)
	Assert.Equals("member-1", refreshed.Me.MemberId)
	Assert.IsTrue(refreshed.SessionKey != session.SessionKey)
}

func (test *SessionControllerTest) TestPostRefreshRefusesAForgedSessionKey() {
	test.signIn("right")
	_, err := (&SessionController{}).PostRefresh(RefreshRequest{SessionKey: "forged"})
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
}

func (test *SessionControllerTest) TestDeleteSessionEndsTheSession() {
	session, _ := test.signIn("right")
	Assert.IsTrue((&SessionController{}).DeleteSession(RefreshRequest{SessionKey: session.SessionKey}) == nil)
}
