import "net/http"

// @Test(HealthTestSetup, {"partition":"tests"})
type OidcIdentityProviderTest struct {
	behaviors *BehaviorContext
}

func (test *OidcIdentityProviderTest) TestIdentifyRefusesARequestWithoutAToken() {
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/search/health", nil)
	Assert.IsTrue((&OidcIdentityProvider{}).Identify(request) == nil)
}

func (test *OidcIdentityProviderTest) TestIdentifyFailsClosedWithoutAVerifier() {
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/search/health", nil)
	request.Header.Set("Authorization", "Bearer anything")
	Assert.IsTrue((&OidcIdentityProvider{}).Identify(request) == nil)
}
