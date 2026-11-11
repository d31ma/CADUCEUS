import "net/http"

// @Test(HealthTestSetup, {"partition":"tests"})
type TokenIdentityProviderTest struct {
	behaviors *BehaviorContext
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesARequestWithoutAToken() {
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/notify/health", nil)
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(request) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyFailsClosedWithoutAVerifier() {
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/notify/health", nil)
	request.Header.Set("Authorization", "Bearer anything")
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(request) == nil)
}
