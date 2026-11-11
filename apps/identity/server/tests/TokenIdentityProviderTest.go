import "net/http"

// @Test(KeycloakTestSetup, {"partition":"tests"})
type TokenIdentityProviderTest struct {
	behaviors *BehaviorContext
}

func (test *TokenIdentityProviderTest) request(token string) *http.Request {
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/identity/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	return request
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesARequestWithoutAToken() {
	KeycloakTestSetup{}.Realm("example.com")
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/identity/me", nil)
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(request) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyFailsClosedWithoutAVerifier() {
	KeycloakTestSetup{}.Unconfigured()
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(test.request("anything")) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyAcceptsAValidKeycloakToken() {
	setup := KeycloakTestSetup{}
	realm := setup.Realm("example.com")
	identity := (&TokenIdentityProvider{}).Identify(test.request(setup.Token(realm, setup.Member())))
	Assert.Equals("example.com/member-1", identity.User)
	Assert.Equals("Member", identity.Roles[0])
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesAForgedToken() {
	KeycloakTestSetup{}.Realm("example.com")
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(test.request("eyJhbGciOiJub25lIn0.eyJzdWIiOiJhbmEifQ.")) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesATokenFromAnotherSigningKey() {
	setup := KeycloakTestSetup{}
	other := setup.Realm("example.com")
	token := setup.Token(other, setup.Member())
	setup.Realm("example.com")
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(test.request(token)) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesAnExpiredToken() {
	setup := KeycloakTestSetup{}
	realm := setup.Realm("example.com")
	member := setup.Member()
	member.ExpiresAt -= 200
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(test.request(setup.Token(realm, member))) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesAnotherAudience() {
	setup := KeycloakTestSetup{}
	realm := setup.Realm("example.com")
	member := setup.Member()
	member.Audience = "account"
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(test.request(setup.Token(realm, member))) == nil)
}
