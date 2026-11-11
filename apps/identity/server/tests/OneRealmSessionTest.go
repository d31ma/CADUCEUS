import (
	"errors"
	"net/http"
)

// @Test(OneRealmTestSetup, {"partition":"tests"})
type OneRealmSessionTest struct {
	behaviors *BehaviorContext
}

func (test *OneRealmSessionTest) signedIn(login string) Session {
	OneRealmTestSetup{}.Start()
	session, err := (&AccountController{}).PostSession(SignInRequest{Login: login, Password: "right"})
	if err != nil {
		panic(err)
	}
	return session
}

func (test *OneRealmSessionTest) TestPostSessionWithThePrefixOpensTheDefaultDomain() {
	session := test.signedIn(" ANA ")
	Assert.Equals("example.com", session.Me.DomainId)
	Assert.Equals("ana", session.Me.Username)
	Assert.Equals("ana@example.com", session.Me.PrimaryAddress)
	Assert.Equals(2, len(session.Domains))
	Assert.IsTrue(session.Domains[0].Admin)
	Assert.IsTrue(session.Domains[0].Default)
	Assert.IsTrue(!session.Domains[1].Default)
	Assert.Equals(int64(60), session.ExpiresIn)
}

func (test *OneRealmSessionTest) TestPostSessionWithAnAddressOpensThatDomain() {
	session := test.signedIn("Ana@Example.ORG")
	Assert.Equals("example.org", session.Me.DomainId)
	Assert.Equals(1, len(session.Me.Roles))
}

func (test *OneRealmSessionTest) TestPostSessionRefusesAWrongPassword() {
	OneRealmTestSetup{}.Start()
	_, err := (&AccountController{}).PostSession(SignInRequest{Login: "ana", Password: "wrong"})
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
}

func (test *OneRealmSessionTest) TestPostSessionRefusesADomainOfAnotherPerson() {
	OneRealmTestSetup{}.Start()
	_, err := (&AccountController{}).PostSession(SignInRequest{Login: "ana@partner.test", Password: "right"})
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden))
	Assert.Equals("not_a_member", forbidden.Code)
}

func (test *OneRealmSessionTest) TestTheMesailTokenIdentifiesTheCaller() {
	session := test.signedIn("ana")
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/identity/me", nil)
	request.Header.Set("Authorization", "Bearer "+session.AccessToken)
	me, err := (&IdentityController{}).GetMe(request)
	Assert.IsTrue(err == nil)
	Assert.Equals("person-ana", me.MemberId)
	Assert.Equals("example.com", me.DomainId)
	identity := (&TokenIdentityProvider{}).Identify(request)
	Assert.Equals("example.com/person-ana", identity.User)
}

func (test *OneRealmSessionTest) TestPostRefreshKeepsTheDomain() {
	session := test.signedIn("ana@example.org")
	refreshed, err := (&AccountController{}).PostRefresh(RefreshRequest{SessionKey: session.SessionKey})
	Assert.IsTrue(err == nil)
	Assert.Equals("example.org", refreshed.Me.DomainId)
}

func (test *OneRealmSessionTest) TestPostDomainSwitchesWithoutAPassword() {
	session := test.signedIn("ana")
	switched, err := (&AccountController{}).PostDomain(DomainSwitch{SessionKey: session.SessionKey, DomainId: "example.org"})
	Assert.IsTrue(err == nil)
	Assert.Equals("example.org", switched.Me.DomainId)
	Assert.IsTrue(switched.SessionKey != session.SessionKey)
}

func (test *OneRealmSessionTest) TestPostDomainRefusesADomainOfAnotherPerson() {
	session := test.signedIn("ana")
	_, err := (&AccountController{}).PostDomain(DomainSwitch{SessionKey: session.SessionKey, DomainId: "partner.test"})
	var notFound *NotFound
	Assert.IsTrue(errors.As(err, &notFound))
}

func (test *OneRealmSessionTest) TestDeleteSessionEndsTheSession() {
	session := test.signedIn("ana")
	Assert.IsTrue((&AccountController{}).DeleteSession(RefreshRequest{SessionKey: session.SessionKey}) == nil)
}

func (test *OneRealmSessionTest) TestGetJwksPublishesTheSigningKey() {
	OneRealmTestSetup{}.Start()
	set, err := (&TokenController{}).GetJwks()
	Assert.IsTrue(err == nil)
	Assert.Equals(1, len(set.Keys))
	Assert.Equals("EC", set.Keys[0].KeyType)
	Assert.Equals("ES256", set.Keys[0].Algorithm)
}

func (test *OneRealmSessionTest) TestPostServiceTokenAnswersADomainToken() {
	OneRealmTestSetup{}.Start()
	token, err := (&TokenController{}).PostServiceToken(ServiceTokenRequest{ClientId: "mesail-service", ClientSecret: "service-secret", DomainId: "*"})
	Assert.IsTrue(err == nil)
	Assert.IsTrue(token.AccessToken != "")
	Assert.Equals(int64(60), token.ExpiresIn)
}

func (test *OneRealmSessionTest) TestPostServiceTokenRefusesAWrongSecret() {
	OneRealmTestSetup{}.Start()
	_, err := (&TokenController{}).PostServiceToken(ServiceTokenRequest{ClientId: "mesail-service", ClientSecret: "wrong", DomainId: "example.com"})
	var unauthenticated *Unauthenticated
	Assert.IsTrue(errors.As(err, &unauthenticated))
}

func (test *OneRealmSessionTest) TestPostServiceTokenRefusesABadDomain() {
	OneRealmTestSetup{}.Start()
	_, err := (&TokenController{}).PostServiceToken(ServiceTokenRequest{ClientId: "mesail-service", ClientSecret: "service-secret", DomainId: "not a domain"})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid))
}

func (test *OneRealmSessionTest) TestGetJwksIsUnavailableWithoutAKey() {
	OneRealmTestSetup{}.Unconfigured()
	_, err := (&TokenController{}).GetJwks()
	var unavailable *Unavailable
	Assert.IsTrue(errors.As(err, &unavailable))
}
