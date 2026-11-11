import (
	"cmp"
	"slices"
	"time"
)

// @Service
type JobRunService struct {
	// @Needs
	fanout *FanoutService
	// @Needs
	pushes *PushService
	// @Needs
	summaries *SummaryService
}

func (service *JobRunService) RunDomains(domainIds []string, budget JobBudget) (JobReport, error) {
	return service.runAll(domainIds, budget)
}

func (service *JobRunService) runAll(domainIds []string, budget JobBudget) (JobReport, error) {
	report := JobReport{}
	written := 0
	failures := []error{}
	for _, domainId := range domainIds {
		step, used, failure := service.step(domainId, service.less(budget, written))
		report, written, failures = service.added(report, step), written+used, append(failures, failure)
	}
	return report, cmp.Or(failures...)
}

func (service *JobRunService) step(domainId string, remaining JobBudget) (JobReport, int, error) {
	if remaining.Writes < 1 || time.Now().After(remaining.Deadline) {
		return JobReport{More: true}, 0, nil
	}
	return service.visit(domainId, remaining)
}

func (service *JobRunService) visit(domainId string, remaining JobBudget) (JobReport, int, error) {
	fannedOut, fanoutFailure := service.fanout.Process(domainId, remaining.Writes)
	afterFanout := service.less(remaining, fannedOut.Written)
	pushed, pushFailure := service.pushes.RunPending(domainId, afterFanout)
	afterPushes := service.less(afterFanout, pushed.Written)
	summarized, summaryFailure := service.summaries.Run(domainId, afterPushes)
	left := service.less(afterPushes, summarized.Written)
	report := JobReport{
		Domains:    1,
		FannedOut:  fannedOut.Done,
		Pushed:     pushed.Pushed,
		Skipped:    pushed.Skipped,
		Summarized: summarized.Done,
		More:       service.anyMore([]JobProgress{fannedOut, pushed, summarized}, left),
	}
	return report, remaining.Writes - left.Writes, cmp.Or(fanoutFailure, pushFailure, summaryFailure)
}

func (service *JobRunService) less(budget JobBudget, written int) JobBudget {
	return JobBudget{Writes: budget.Writes - written, Deadline: budget.Deadline}
}

func (service *JobRunService) anyMore(phases []JobProgress, left JobBudget) bool {
	return left.Writes < 1 || slices.ContainsFunc(phases, func(phase JobProgress) bool { return phase.More })
}

func (service *JobRunService) added(total JobReport, step JobReport) JobReport {
	return JobReport{
		Domains:    total.Domains + step.Domains,
		FannedOut:  total.FannedOut + step.FannedOut,
		Pushed:     total.Pushed + step.Pushed,
		Skipped:    total.Skipped + step.Skipped,
		Summarized: total.Summarized + step.Summarized,
		More:       total.More || step.More,
	}
}
