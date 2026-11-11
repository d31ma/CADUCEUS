import (
	"cmp"
	"errors"
	"slices"
)

// @Service
type PersonLeavingService struct {
	// @Needs
	people *PeopleStorageService
	// @Needs
	writer *MemberWriterService
}

func (service *PersonLeavingService) Continue(budget *WriteBudget) error {
	people, failure := service.people.Leaving()
	left := BatchUtils{}.Each(len(people), func(index int) error { return service.leaveEachDomain(people[index], budget) })
	return cmp.Or(failure, left)
}

func (service *PersonLeavingService) leaveEachDomain(person PersonDocument, budget *WriteBudget) error {
	failures := []error{}
	for _, domainId := range slices.Clone(person.DomainIds) {
		var failure error
		person, failure = service.leave(person, domainId, budget)
		failures = append(failures, failure)
	}
	return errors.Join(failures...)
}

func (service *PersonLeavingService) leave(person PersonDocument, domainId string, budget *WriteBudget) (PersonDocument, error) {
	if !budget.Take(service.leaveWrites()) {
		return person, nil
	}
	return service.leaveAndStore(person, domainId)
}

func (service *PersonLeavingService) leaveAndStore(person PersonDocument, domainId string) (PersonDocument, error) {
	failure := service.writer.LeaveDomain(person.PersonId, domainId)
	return service.storeUnless(failure, PersonRules{}.Without(person, domainId))
}

func (service *PersonLeavingService) storeUnless(failure error, person PersonDocument) (PersonDocument, error) {
	if failure != nil {
		return person, failure
	}
	return service.people.Store(person)
}

func (service *PersonLeavingService) leaveWrites() int {
	return 4
}
