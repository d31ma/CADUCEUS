import "cmp"

// @Service
type AdmissionService struct {
	// @Needs
	callers *CallerService
	// @Needs
	people *PeopleStorageService
	// @Needs
	tenants *TenantService
}

func (service *AdmissionService) Admit(authorization string) Admission {
	caller, refused := service.callers.Caller(authorization)
	person, _, failure := service.personUnless(refused, caller.MemberId)
	return Admission{Caller: caller, Refusal: service.admitted(cmp.Or(refused, failure), person, caller.DomainId)}
}

func (service *AdmissionService) personUnless(refused error, personId string) (PersonDocument, bool, error) {
	if refused != nil {
		return PersonDocument{}, false, refused
	}
	return service.people.ByPersonId(personId)
}

func (service *AdmissionService) admitted(failure error, person PersonDocument, domainId string) error {
	if failure != nil {
		return failure
	}
	return service.memberRefusal(person, domainId)
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
