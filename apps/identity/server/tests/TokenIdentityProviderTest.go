import (
	"net/http"

	"mesail.dev/packages/oidc"
)

// @Test(OneRealmTestSetup, {"partition":"tests"})
type TokenIdentityProviderTest struct {
	behaviors *BehaviorContext
}

func (test *TokenIdentityProviderTest) request(token string) *http.Request {
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/identity/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	return request
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesARequestWithoutAToken() {
	OneRealmTestSetup{}.Start()
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/identity/me", nil)
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(request) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyFailsClosedWithoutAVerifier() {
	OneRealmTestSetup{}.Unconfigured()
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(test.request("anything")) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyAcceptsAValidMesailToken() {
	setup := OneRealmTestSetup{}
	key := setup.Start()
	identity := (&TokenIdentityProvider{}).Identify(test.request(setup.Token(key, setup.Member())))
	Assert.Equals("example.com/person-ana", identity.User)
	Assert.Equals("Member", identity.Roles[0])
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesAForgedToken() {
	OneRealmTestSetup{}.Start()
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(test.request("eyJhbGciOiJub25lIn0.eyJzdWIiOiJhbmEifQ.")) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesATokenFromAnotherSigningKey() {
	setup := OneRealmTestSetup{}
	setup.Start()
	other, _ := oidc.SigningKey{}.Generated()
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(test.request(setup.Token(other, setup.Member()))) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesAnExpiredToken() {
	setup := OneRealmTestSetup{}
	key := setup.Start()
	member := setup.Member()
	member.ExpiresAt -= 200
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(test.request(setup.Token(key, member))) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesAnotherAudience() {
	setup := OneRealmTestSetup{}
	key := setup.Start()
	member := setup.Member()
	member.Audience = oidc.AudienceList{"account"}
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(test.request(setup.Token(key, member))) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesAKeycloakToken() {
	setup := OneRealmTestSetup{}
	key := setup.Start()
	member := setup.Member()
	member.AuthorizedParty = "mesail-identity"
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(test.request(setup.Token(key, member))) == nil)
}
