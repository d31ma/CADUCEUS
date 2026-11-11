import (
	"errors"

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
	Assert.IsTrue(err == nil, "operator refused")
}

func (test *CallerServiceTest) TestAnOperatorRoleInsideAMailDomainIsRefused() {
	err := (&CallerService{}).Operator(oidc.Caller{DomainId: "example.com", Roles: []string{"Operator"}})
	var forbidden *Forbidden
	Assert.IsTrue(errors.As(err, &forbidden), "want Forbidden")
}

func (test *CallerServiceTest) TestAServiceOfTheTargetDomainIsAllowed() {
	Assert.IsTrue((&CallerService{}).ServiceFor(oidc.Caller{DomainId: "example.com"}, "Example.COM") == nil, "service refused")
}

func (test *CallerServiceTest) TestAServiceOfTheOperatorRealmIsAllowedEverywhere() {
	Assert.IsTrue((&CallerService{}).ServiceFor(oidc.Caller{DomainId: "mesail-operators"}, "example.com") == nil, "service refused")
}

func (test *CallerServiceTest) TestAServiceOfAnotherDomainGetsNotFound() {
	err := (&CallerService{}).ServiceFor(oidc.Caller{DomainId: "example.org"}, "example.com")
	var missing *NotFound
	Assert.IsTrue(errors.As(err, &missing), "want NotFound")
}
