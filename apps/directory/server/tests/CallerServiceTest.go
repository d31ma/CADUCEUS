import (
	"errors"
	"fmt"
	"os"
	"strings"

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

func (test *CallerServiceTest) TestAnOperatorWithoutADomainIsAccepted() {
	err := (&CallerService{}).Operator(oidc.Caller{DomainId: "*", Roles: []string{"Operator"}})
	Assert.Equals("<nil>", fmt.Sprint(err))
}

func (test *CallerServiceTest) TestAnOperatorWithAnActiveDomainIsAccepted() {
	Assert.Equals("<nil>", fmt.Sprint((&CallerService{}).Operator(oidc.Caller{DomainId: "example.com", Roles: []string{"Operator"}})))
}

func (test *CallerServiceTest) TestACallerWithoutTheOperatorRoleIsRefused() {
	err := (&CallerService{}).Operator(oidc.Caller{DomainId: "*", Roles: []string{"Member"}})
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
}

func (test *CallerServiceTest) TestAServiceOfTheTargetDomainIsAllowed() {
	Assert.Equals("<nil>", fmt.Sprint((&CallerService{}).ServiceFor(oidc.Caller{DomainId: "example.com"}, "Example.COM")))
}

func (test *CallerServiceTest) TestTheOldOperatorRealmNameIsNotInstallWideForAService() {
	err := (&CallerService{}).ServiceFor(oidc.Caller{DomainId: "mesail-operators"}, "example.com")
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *CallerServiceTest) TestTheOldOperatorRealmNameIsNotInstallWide() {
	err := (&CallerService{}).InstallWide(oidc.Caller{DomainId: "mesail-operators"})
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}

func (test *CallerServiceTest) TestAMesailTokenNamesTheCallerAndItsDomain() {
	setup := DirectoryTestSetup{}
	realm := setup.SignedRealm()
	os.Setenv("YON_APPLICATION_JSON", setup.ApplicationJson(realm, "http://127.0.0.1:1"))
	caller, err := (&CallerService{}).Caller(setup.PersonBearer(realm, "person-1", "example.com", "mesail-member"))
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("person-1", caller.MemberId)
	Assert.Equals("example.com", caller.DomainId)
	Assert.Equals("Member", strings.Join(caller.Roles, ","))
}

func (test *CallerServiceTest) TestAnInstallWideServiceTokenIsInstallWide() {
	setup := DirectoryTestSetup{}
	realm := setup.SignedRealm()
	os.Setenv("YON_APPLICATION_JSON", setup.ApplicationJson(realm, "http://127.0.0.1:1"))
	caller, err := (&CallerService{}).Caller(setup.ServiceBearer(realm, oidc.InstallWide{}.DomainId()))
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("<nil>", fmt.Sprint((&CallerService{}).InstallWide(caller)))
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
