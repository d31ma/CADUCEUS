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

func (service *DomainRemovalService) Continue(budget *WriteBudget) error {
	domains, failure := service.domains.Removing()
	finished := BatchUtils{}.Each(len(domains), func(index int) error { return service.finish(domains[index], budget) })
	return cmp.Or(failure, finished)
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
	switch {
	case failure != nil:
		return failure
	case !budget.Take(service.dropWrites()):
		return nil
	}
	return service.drop(domain)
}

func (service *DomainRemovalService) drop(domain DomainDocument) error {
	failure := service.collections.Drop(domain.DomainId)
	return Steps{}.Unless(failure, func() error { return service.domains.Delete(domain) })
}

func (service *DomainRemovalService) oneWrite() int {
	return 1
}

func (service *DomainRemovalService) dropWrites() int {
	return 3
}
