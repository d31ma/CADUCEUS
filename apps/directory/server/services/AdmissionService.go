import (
	"cmp"

	"mesail.dev/packages/oidc"
)

// @Service
type AdmissionService struct {
	// @Needs
	callers *CallerService
	// @Needs
	tokenMembers *TokenMemberService
	// @Needs
	people *PeopleStorageService
	// @Needs
	tenants *TenantService
}

func (service *AdmissionService) Admit(authorization string) Admission {
	caller, refused := service.callers.Caller(authorization)
	person, found, failure := service.personUnless(refused, caller.MemberId)
	return Admission{Caller: caller, Refusal: service.admitted(cmp.Or(refused, failure), found, person, caller)}
}

func (service *AdmissionService) personUnless(refused error, personId string) (PersonDocument, bool, error) {
	if refused != nil {
		return PersonDocument{}, false, refused
	}
	return service.people.ByPersonId(personId)
}

func (service *AdmissionService) admitted(failure error, found bool, person PersonDocument, caller oidc.Caller) error {
	switch {
	case failure != nil:
		return failure
	case !found:
		return service.remembered(caller)
	}
	return service.memberRefusal(person, caller.DomainId)
}

func (service *AdmissionService) remembered(caller oidc.Caller) error {
	_, failure := service.tokenMembers.Remember(caller)
	return failure
}

func (service *AdmissionService) memberRefusal(person PersonDocument, domainId string) error {
	_, refused := service.tenants.Require(domainId)
	return cmp.Or(refused, service.member(person, domainId))
}

func (service *AdmissionService) member(person PersonDocument, domainId string) error {
	switch {
	case person.State != service.active(), !(PersonRules{}).Member(person, domainId):
		return &Forbidden{Message: "you are not a member of this domain"}
	}
	return nil
}

func (service *AdmissionService) active() string {
	return "active"
}
