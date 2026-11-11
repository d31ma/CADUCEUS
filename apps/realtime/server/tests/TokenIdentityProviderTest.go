import "net/http"

// @Test(GatewayTestSetup, {"partition":"tests"})
type TokenIdentityProviderTest struct {
	behaviors *BehaviorContext
}

func (test *TokenIdentityProviderTest) request() *http.Request {
	request, _ := http.NewRequest(http.MethodPost, "/api/v1/realtime/gateway/message", nil)
	return request
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesARequestWithoutAToken() {
	GatewayTestSetup{}.Realm("http://127.0.0.1:1")
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(test.request()) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesAForgedToken() {
	request := test.request()
	request.Header.Set("Authorization", "Bearer anything")
	GatewayTestSetup{}.Realm("http://127.0.0.1:1")
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(request) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyAcceptsAMemberToken() {
	setup := GatewayTestSetup{}
	realm := setup.Realm("http://127.0.0.1:1")
	request := test.request()
	request.Header.Set("Authorization", "Bearer "+setup.Token(realm, []string{"mesail-member"}))
	identity := (&TokenIdentityProvider{}).Identify(request)
	Assert.Equals("example.com/member-1", identity.User)
}

func (test *TokenIdentityProviderTest) TestIdentifyNamesTheGateway() {
	setup := GatewayTestSetup{}
	setup.Realm("http://127.0.0.1:1")
	request := test.request()
	request.Header.Set("X-Realtime-Gateway", setup.Secret())
	identity := (&TokenIdentityProvider{}).Identify(request)
	Assert.Equals("gateway", identity.User)
	Assert.Equals("Gateway", identity.Roles[0])
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesAWrongGatewaySecret() {
	GatewayTestSetup{}.Realm("http://127.0.0.1:1")
	request := test.request()
	request.Header.Set("X-Realtime-Gateway", "wrong-secret-that-is-long-enough-to-pass")
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(request) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyDoesNotFallBackToTheTokenWithAWrongGatewaySecret() {
	setup := GatewayTestSetup{}
	realm := setup.Realm("http://127.0.0.1:1")
	request := test.request()
	request.Header.Set("X-Realtime-Gateway", "wrong")
	request.Header.Set("Authorization", "Bearer "+setup.Token(realm, []string{"mesail-member"}))
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(request) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesAShortGatewaySecret() {
	setup := GatewayTestSetup{}
	realm := setup.Realm("http://127.0.0.1:1")
	setup.UseSettings(realm, "http://127.0.0.1:1", "short")
	request := test.request()
	request.Header.Set("X-Realtime-Gateway", "short")
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(request) == nil)
}
