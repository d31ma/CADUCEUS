import (
	"cmp"
	"slices"

	"mesail.dev/packages/oidc"
)

// @Service
type PersonAccessService struct {
	// @Needs
	people *PeopleStorageService
	// @Needs
	accounts *AccountAdminService
}

func (service *PersonAccessService) ManagerOf(caller oidc.Caller) (AccountManager, error) {
	own, _, failure := service.people.ByPersonId(caller.MemberId)
	return AccountManager{Operator: service.operator(caller), AdminDomainIds: own.AdminDomainIds}, failure
}

func (service *PersonAccessService) Check(caller oidc.Caller, person PersonDocument) error {
	manager, failure := service.ManagerOf(caller)
	failure = cmp.Or(failure, service.managedBy(manager, person.DomainIds))
	operator, roleFailure := service.operatorUnless(failure, manager.Operator, person.PersonId)
	return cmp.Or(roleFailure, service.notAnOperator(operator))
}

func (service *PersonAccessService) managedBy(manager AccountManager, domainIds []string) error {
	if !manager.Manages(domainIds) {
		return service.forbidden()
	}
	return nil
}

func (service *PersonAccessService) operatorUnless(failure error, callerIsOperator bool, personId string) (bool, error) {
	switch {
	case failure != nil:
		return false, failure
	case callerIsOperator:
		return false, nil
	}
	return service.accounts.IsOperator(personId)
}

func (service *PersonAccessService) notAnOperator(operator bool) error {
	if operator {
		return service.forbidden()
	}
	return nil
}

func (service *PersonAccessService) operator(caller oidc.Caller) bool {
	return slices.Contains(caller.Roles, service.operatorRole())
}

func (service *PersonAccessService) forbidden() error {
	return &Forbidden{Message: "only an Operator or an admin of every domain of this person may change the account"}
}

func (service *PersonAccessService) operatorRole() string {
	return "Operator"
}
