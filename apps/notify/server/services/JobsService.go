import (
	"cmp"
	"time"
)

// @Service
type JobsService struct {
	// @Needs
	callers *CallerService
	// @Needs
	domains *DomainRegistryService
	// @Needs
	runner *JobRunService
}

func (service *JobsService) Run(authorization string) (JobReport, error) {
	failure := service.callers.Operator(authorization)
	domainIds, failure := service.domainsUnless(failure)
	report, problem := service.runner.RunDomains(domainIds, JobBudget{Writes: service.writeLimit(), Deadline: time.Now().Add(service.timeLimit())})
	return report, cmp.Or(failure, problem)
}

func (service *JobsService) domainsUnless(failure error) ([]string, error) {
	if failure != nil {
		return []string{}, failure
	}
	return service.domains.All(service.domainLimit())
}

func (service *JobsService) writeLimit() int {
	return 25
}

func (service *JobsService) timeLimit() time.Duration {
	return 15 * time.Second
}

func (service *JobsService) domainLimit() int {
	return 1000
}
