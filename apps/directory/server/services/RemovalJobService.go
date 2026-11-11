import (
	"errors"
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
	budget := &WriteBudget{Left: service.writeLimit(), Deadline: time.Now().Add(service.timeLimit())}
	peopleFailure := service.people.Continue(budget)
	domainFailure := service.domains.Continue(budget)
	return budget.Report(), errors.Join(peopleFailure, domainFailure)
}

func (service *RemovalJobService) writeLimit() int {
	return 25
}

func (service *RemovalJobService) timeLimit() time.Duration {
	return 20 * time.Second
}
