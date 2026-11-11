import (
	"errors"
	"fmt"

	"mesail.dev/packages/oidc"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type CallerServiceTest struct {
	behaviors *BehaviorContext
}

func (test *CallerServiceTest) TestATokenThatCannotBeVerifiedIsRefused() {
	_, err := (&CallerService{}).Caller("Bearer forged")
	var refused *Unauthenticated
	Assert.IsTrue(errors.As(err, &refused), "want Unauthenticated")
}

func (test *CallerServiceTest) TestAnOperatorFromTheOperatorRealmIsAccepted() {
	err := (&CallerService{}).Operator(oidc.Caller{DomainId: "mesail-operators", Roles: []string{"Operator"}})
	Assert.Equals("<nil>", fmt.Sprint(err))
}

func (test *CallerServiceTest) TestAnOperatorWithAnActiveDomainIsAccepted() {
	Assert.Equals("<nil>", fmt.Sprint((&CallerService{}).Operator(oidc.Caller{DomainId: "example.com", Roles: []string{"Operator"}})))
}

func (test *CallerServiceTest) TestACallerWithoutTheOperatorRoleIsRefused() {
	err := (&CallerService{}).Operator(oidc.Caller{DomainId: "mesail-operators", Roles: []string{"Member"}})
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
}

func (test *CallerServiceTest) TestAServiceOfTheTargetDomainIsAllowed() {
	Assert.Equals("<nil>", fmt.Sprint((&CallerService{}).ServiceFor(oidc.Caller{DomainId: "example.com"}, "Example.COM")))
}

func (test *CallerServiceTest) TestAServiceOfTheOperatorRealmIsAllowedEverywhere() {
	Assert.Equals("<nil>", fmt.Sprint((&CallerService{}).ServiceFor(oidc.Caller{DomainId: "mesail-operators"}, "example.com")))
}

func (test *CallerServiceTest) TestAServiceOfAnotherDomainGetsNotFound() {
	err := (&CallerService{}).ServiceFor(oidc.Caller{DomainId: "example.org"}, "example.com")
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *CallerServiceTest) TestAnInstallWideServiceIsAllowedEverywhere() {
	Assert.Equals("<nil>", fmt.Sprint((&CallerService{}).ServiceFor(oidc.Caller{DomainId: "*"}, "example.com")))
}

func (test *CallerServiceTest) TestAnOperatorReadsAnyDomain() {
	Assert.Equals("<nil>", fmt.Sprint((&CallerService{}).ServiceFor(oidc.Caller{DomainId: "example.org", Roles: []string{"Operator"}}, "example.com")))
}

func (test *CallerServiceTest) TestInstallWideAcceptsTheInstallWideDomain() {
	Assert.Equals("<nil>", fmt.Sprint((&CallerService{}).InstallWide(oidc.Caller{DomainId: "*"})))
}

func (test *CallerServiceTest) TestInstallWideRefusesAMailDomain() {
	err := (&CallerService{}).InstallWide(oidc.Caller{DomainId: "example.com"})
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}
