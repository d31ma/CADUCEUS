import (
	"os"
	"strings"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type ServiceTokenServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ServiceTokenServiceTest) TestTokenForComesWithoutTheBearerPrefix() {
	setup := NotifyTestSetup{}
	member, operators := setup.Realms(setup.Domain())
	setup.Keycloak(member, operators)
	token, err := (&ServiceTokenService{}).TokenFor(operators.Name)
	Assert.IsTrue(err == nil, "token failed")
	Assert.Equals(setup.Service(operators)[len("Bearer "):][:20], token[:20])
}

func (test *ServiceTokenServiceTest) TestTokenForServesTheRealmOfADomain() {
	setup := NotifyTestSetup{}
	member, operators := setup.Realms(setup.Domain())
	setup.Keycloak(member, operators)
	token, err := (&ServiceTokenService{}).TokenFor(member.Name)
	Assert.IsTrue(err == nil, "token failed")
	Assert.IsTrue(!strings.HasPrefix(token, "Bearer "), "the token kept its prefix")
}

func (test *ServiceTokenServiceTest) TestTokenForFailsWithoutSettings() {
	os.Setenv("YON_APPLICATION_JSON", "{}")
	_, err := (&ServiceTokenService{}).TokenFor("example.org")
	Assert.IsTrue(err != nil, "a token came without settings")
}
