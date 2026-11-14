import (
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

func (service *PersonLeavingService) Continue(offset int, budget *WriteBudget) error {
	people, failure := service.people.Leaving()
	return TurnUtils{}.Visit(service.people.Ids(people), failure, offset, func(index int) error { return service.leaveEachDomain(people[index], budget) })
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
	gone, failure := service.writer.Gone(person.PersonId, domainId)
	return service.nextStep(gone, failure, PersonLeave{Person: person, DomainId: domainId}, budget)
}

func (service *PersonLeavingService) nextStep(gone bool, failure error, leave PersonLeave, budget *WriteBudget) (PersonDocument, error) {
	switch {
	case failure != nil:
		return leave.Person, failure
	case gone:
		return service.storeWithin(leave, budget)
	}
	return leave.Person, budget.Spend(service.leaveWrites(), func() error { return service.leaveNow(leave, budget) })
}

func (service *PersonLeavingService) leaveNow(leave PersonLeave, budget *WriteBudget) error {
	budget.MoreLater()
	return service.writer.LeaveDomain(leave.Person.PersonId, leave.DomainId)
}

func (service *PersonLeavingService) storeWithin(leave PersonLeave, budget *WriteBudget) (PersonDocument, error) {
	result := leave.Person
	failure := budget.Spend(service.storeWrites(), func() error {
		stored, storeFailure := service.people.Store(PersonRules{}.Without(leave.Person, leave.DomainId))
		result = stored
		return storeFailure
	})
	return result, failure
}

func (service *PersonLeavingService) leaveWrites() int {
	return 3
}

func (service *PersonLeavingService) storeWrites() int {
	return 1
}
