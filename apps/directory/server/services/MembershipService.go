import (
	"cmp"
	"slices"
	"strings"

	"mesail.dev/packages/oidc"
)

// @Service
type MembershipService struct {
	// @Needs
	tenants *TenantService
	// @Needs
	enroll *PersonEnrollService
	// @Needs
	people *PeopleStorageService
	// @Needs
	writer *MemberWriterService
	// @Needs
	access *PersonAccessService
}

func (service *MembershipService) Add(caller oidc.Caller, domainId string, request AddMemberRequest) (AddedMember, error) {
	domainId = strings.ToLower(strings.TrimSpace(domainId))
	_, refused := service.tenants.Require(domainId)
	person, existing, failure := service.enrollUnless(refused, request)
	failure = cmp.Or(failure, service.notYetMember(person, domainId))
	stored, failure := service.storeUnless(failure, service.withDomain(person, domainId, request.Admin))
	member, failure := service.joinUnless(failure, stored, domainId)
	manager, managerFailure := service.access.ManagerOf(caller)
	summary := MemberSummaries{}.Admin(member, manager.Manages(stored.DomainIds))
	return AddedMember{Member: summary, ExistingPerson: existing}, cmp.Or(failure, managerFailure)
}

func (service *MembershipService) Change(caller oidc.Caller, memberId string, change MemberChange) (MemberSummary, error) {
	member, failure := service.writer.Active(caller.DomainId, memberId)
	failure = service.adminLeftUnless(failure, member, change.Admin)
	person, found, readFailure := service.people.ByPersonId(memberId)
	person, failure = service.storeFoundUnless(cmp.Or(failure, readFailure), found, PersonRules{}.WithAdmin(person, caller.DomainId, change.Admin))
	member.Admin = change.Admin
	saved, failure := service.updateUnless(failure, member)
	manager, managerFailure := service.access.ManagerOf(caller)
	return MemberSummaries{}.Admin(saved, manager.Manages(person.DomainIds)), cmp.Or(failure, managerFailure)
}

func (service *MembershipService) Remove(caller oidc.Caller, memberId string) error {
	member, failure := service.writer.Active(caller.DomainId, memberId)
	failure = service.adminLeftUnless(failure, member, false)
	person, found, readFailure := service.people.ByPersonId(memberId)
	_, failure = service.storeFoundUnless(cmp.Or(failure, readFailure), found, PersonRules{}.Without(person, caller.DomainId))
	return service.leaveUnless(failure, member)
}

func (service *MembershipService) enrollUnless(refused error, request AddMemberRequest) (PersonDocument, bool, error) {
	if refused != nil {
		return PersonDocument{}, false, refused
	}
	return service.enroll.Enroll(request)
}

func (service *MembershipService) storeUnless(failure error, person PersonDocument) (PersonDocument, error) {
	if failure != nil {
		return person, failure
	}
	return service.people.Store(person)
}

func (service *MembershipService) joinUnless(failure error, person PersonDocument, domainId string) (MemberDocument, error) {
	if failure != nil {
		return MemberDocument{}, failure
	}
	return service.writer.Join(person, domainId, slices.Contains(person.AdminDomainIds, domainId))
}

func (service *MembershipService) adminLeftUnless(failure error, member MemberDocument, admin bool) error {
	switch {
	case failure != nil:
		return failure
	case !member.Admin, admin:
		return nil
	}
	return service.writer.OtherAdmin(member)
}

func (service *MembershipService) storeFoundUnless(failure error, found bool, person PersonDocument) (PersonDocument, error) {
	switch {
	case failure != nil:
		return person, failure
	case !found:
		return person, nil
	}
	return service.people.Store(person)
}

func (service *MembershipService) updateUnless(failure error, member MemberDocument) (MemberDocument, error) {
	if failure != nil {
		return member, failure
	}
	return service.writer.Update(member)
}

func (service *MembershipService) leaveUnless(failure error, member MemberDocument) error {
	if failure != nil {
		return failure
	}
	return service.writer.Leave(member)
}

func (service *MembershipService) withDomain(person PersonDocument, domainId string, admin bool) PersonDocument {
	return PersonRules{}.WithAdmin(PersonRules{}.With(person, domainId), domainId, admin)
}

func (service *MembershipService) notYetMember(person PersonDocument, domainId string) error {
	if (PersonRules{}).Member(person, domainId) {
		return &Duplicate{Message: person.Username + " is already a member of " + domainId}
	}
	return nil
}
