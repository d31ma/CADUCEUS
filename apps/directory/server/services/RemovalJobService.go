import (
	"errors"
	"math/rand/v2"
	"time"
)

// @Service
type RemovalJobService struct {
	// @Needs
	domains *DomainRemovalService
	// @Needs
	people *PersonLeavingService
}

func (service *RemovalJobService) RunOnce() (RemovalReport, error) {
	return service.runFrom(rand.Int(), rand.IntN(service.queues()) == 0)
}

func (service *RemovalJobService) runFrom(offset int, peopleFirst bool) (RemovalReport, error) {
	budget := &WriteBudget{Left: service.writeLimit(), Deadline: time.Now().Add(service.timeLimit())}
	failure := service.continueBoth(offset, peopleFirst, budget)
	return budget.Report(), failure
}

func (service *RemovalJobService) continueBoth(offset int, peopleFirst bool, budget *WriteBudget) error {
	if peopleFirst {
		return service.peopleThenDomains(offset, budget)
	}
	return service.domainsThenPeople(offset, budget)
}

func (service *RemovalJobService) peopleThenDomains(offset int, budget *WriteBudget) error {
	peopleFailure := service.people.Continue(offset, budget)
	domainFailure := service.domains.Continue(budget, offset)
	return errors.Join(peopleFailure, domainFailure)
}

func (service *RemovalJobService) domainsThenPeople(offset int, budget *WriteBudget) error {
	domainFailure := service.domains.Continue(budget, offset)
	peopleFailure := service.people.Continue(offset, budget)
	return errors.Join(domainFailure, peopleFailure)
}

func (service *RemovalJobService) queues() int {
	return 2
}

func (service *RemovalJobService) writeLimit() int {
	return 3
}

func (service *RemovalJobService) timeLimit() time.Duration {
	return time.Second
}
