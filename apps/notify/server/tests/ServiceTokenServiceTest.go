import (
	"os"

	"mesail.dev/packages/oidc"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type ServiceTokenServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ServiceTokenServiceTest) TestTokenForComesWithoutTheBearerPrefix() {
	setup := NotifyTestSetup{}
	member, installWide := setup.Realms(setup.Domain())
	setup.IdentityServer(member, installWide)
	token, err := (&ServiceTokenService{}).TokenFor(installWide.Domain)
	caller, refused := (&CallerService{}).Caller("Bearer " + token)
	Assert.IsTrue(err == nil && refused == nil, "token failed")
	Assert.Equals(oidc.InstallWide{}.DomainId(), caller.DomainId)
}

func (test *ServiceTokenServiceTest) TestTokenForServesTheDomainItNames() {
	setup := NotifyTestSetup{}
	member, installWide := setup.Realms(setup.Domain())
	setup.IdentityServer(member, installWide)
	token, err := (&ServiceTokenService{}).TokenFor(member.Domain)
	caller, refused := (&CallerService{}).Caller("Bearer " + token)
	Assert.IsTrue(err == nil && refused == nil, "token failed")
	Assert.Equals(member.Domain, caller.DomainId)
}

func (test *ServiceTokenServiceTest) TestTokenForFailsWithoutSettings() {
	os.Setenv("YON_APPLICATION_JSON", "{}")
	_, err := (&ServiceTokenService{}).TokenFor("example.org")
	Assert.IsTrue(err != nil, "a token came without settings")
}
