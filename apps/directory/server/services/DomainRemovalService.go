import "cmp"

// @Service
type DomainRemovalService struct {
	// @Needs
	domains *DomainStorageService
	// @Needs
	people *PeopleStorageService
	// @Needs
	localParts *LocalPartStorageService
	// @Needs
	reservations *ReservationService
	// @Needs
	collections *TenantCollectionsService
}

func (service *DomainRemovalService) Continue(budget *WriteBudget, offset int) error {
	domains, failure := service.domains.Removing()
	return TurnUtils{}.Visit(service.domains.Ids(domains), failure, offset, func(index int) error { return service.finish(domains[index], budget) })
}

func (service *DomainRemovalService) finish(domain DomainDocument, budget *WriteBudget) error {
	peopleFailure := service.releasePeople(domain.DomainId, budget)
	addressFailure := service.releaseAddresses(domain.DomainId, budget)
	return service.dropUnless(cmp.Or(peopleFailure, addressFailure), domain, budget)
}

func (service *DomainRemovalService) releasePeople(domainId string, budget *WriteBudget) error {
	people, failure := service.people.InDomain(domainId)
	released := BatchUtils{}.Each(len(people), func(index int) error {
		return budget.Spend(service.oneWrite(), func() error { return service.releasePerson(people[index], domainId) })
	})
	return cmp.Or(failure, released)
}

func (service *DomainRemovalService) releasePerson(person PersonDocument, domainId string) error {
	_, failure := service.people.Store(PersonRules{}.Without(person, domainId))
	return failure
}

func (service *DomainRemovalService) releaseAddresses(domainId string, budget *WriteBudget) error {
	documents, failure := service.localParts.ForAddressesIn(domainId)
	released := BatchUtils{}.Each(len(documents), func(index int) error {
		return budget.Spend(service.oneWrite(), func() error { return service.reservations.ReleaseDomain(documents[index], domainId) })
	})
	return cmp.Or(failure, released)
}

func (service *DomainRemovalService) dropUnless(failure error, domain DomainDocument, budget *WriteBudget) error {
	if failure != nil {
		return failure
	}
	return service.dropInParts(domain, budget)
}

func (service *DomainRemovalService) dropInParts(domain DomainDocument, budget *WriteBudget) error {
	failure := service.collections.Drop(domain.DomainId, budget)
	return Steps{}.Unless(failure, func() error {
		return budget.Spend(service.oneWrite(), func() error { return service.domains.Delete(domain) })
	})
}

func (service *DomainRemovalService) oneWrite() int {
	return 1
}
