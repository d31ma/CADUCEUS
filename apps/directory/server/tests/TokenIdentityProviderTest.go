import (
	"net/http"
	"os"
	"strings"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type TokenIdentityProviderTest struct {
	behaviors *BehaviorContext
}

func (test *TokenIdentityProviderTest) TestIdentifyRefusesARequestWithoutAToken() {
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/directory/health", nil)
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(request) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyFailsClosedWithoutAVerifier() {
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/directory/health", nil)
	request.Header.Set("Authorization", "Bearer anything")
	Assert.IsTrue((&TokenIdentityProvider{}).Identify(request) == nil)
}

func (test *TokenIdentityProviderTest) TestIdentifyNamesTheDomainAndRolesOfAMesailToken() {
	setup := DirectoryTestSetup{}
	realm := setup.SignedRealm()
	os.Setenv("YON_APPLICATION_JSON", setup.ApplicationJson(realm, "http://127.0.0.1:1"))
	request, _ := http.NewRequest(http.MethodGet, "/api/v1/directory/health", nil)
	request.Header.Set("Authorization", setup.ServiceBearer(realm, "*"))
	identity := (&TokenIdentityProvider{}).Identify(request)
	Assert.Equals("*/service", identity.User)
	Assert.Equals("Service", strings.Join(identity.Roles, ","))
}
