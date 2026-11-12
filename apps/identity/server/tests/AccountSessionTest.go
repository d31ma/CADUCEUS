import "errors"

// @Test(OneRealmTestSetup, {"partition":"tests"})
type AccountSessionTest struct {
	behaviors *BehaviorContext
}

func (test *AccountSessionTest) signIn(password string) (Session, error) {
	OneRealmTestSetup{}.Start()
	return OneRealmTestSetup{}.Account().PostSession(SignInRequest{Login: "ana", Password: password, OneTimeCode: "123456"})
}

func (test *AccountSessionTest) TestPostSessionAnswersTheSession() {
	session, err := test.signIn("right")
	Assert.IsTrue(err == nil)
	Assert.Equals("person-ana", session.Me.MemberId)
	Assert.IsTrue(session.AccessToken != "")
	Assert.IsTrue(session.SessionKey != "")
}

func (test *AccountSessionTest) TestPostSessionReportsALockedAccount() {
	_, err := test.signIn("locked")
	var limited *RateLimited
	Assert.IsTrue(errors.As(err, &limited))
	Assert.Equals(503, limited.Number())
}

func (test *AccountSessionTest) TestPostSessionIsUnavailableWithoutSettings() {
	OneRealmTestSetup{}.Unconfigured()
	_, err := OneRealmTestSetup{}.Account().PostSession(SignInRequest{Login: "ana", Password: "right"})
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable))
	Assert.Equals(504, unavailable.Number())
}

func (test *AccountSessionTest) TestPostRefreshAnswersANewSession() {
	session, _ := test.signIn("right")
	refreshed, err := OneRealmTestSetup{}.Account().PostRefresh(RefreshRequest{SessionKey: session.SessionKey})
	Assert.IsTrue(err == nil)
	Assert.Equals("person-ana", refreshed.Me.MemberId)
	Assert.IsTrue(refreshed.SessionKey != session.SessionKey)
}

func (test *AccountSessionTest) TestPostRefreshRefusesAForgedSessionKey() {
	test.signIn("right")
	_, err := OneRealmTestSetup{}.Account().PostRefresh(RefreshRequest{SessionKey: "forged"})
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
	Assert.Equals(501, unauthenticated.Number())
}
