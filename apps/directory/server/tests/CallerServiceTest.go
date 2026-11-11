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

func (test *CallerServiceTest) TestAnOperatorRoleInsideAMailDomainIsRefused() {
	err := (&CallerService{}).Operator(oidc.Caller{DomainId: "example.com", Roles: []string{"Operator"}})
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
