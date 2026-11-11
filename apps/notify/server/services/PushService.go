import (
	"cmp"
	"slices"
	"time"

	"mesail.dev/packages/notify"
	"mesail.dev/packages/webpush"
)

// @Service
type PushService struct {
	// @Needs
	store *NotificationStoreService
	// @Needs
	presence *PresenceService
	// @Needs
	targets *PushTargetsService
	// @Needs
	sender *PushSenderService
	// @Needs
	states *NotificationStatesService
}

func (service *PushService) Deliver(domainId string, records []NotificationRecord, writeBudget int) (JobProgress, error) {
	budget := JobBudget{Writes: writeBudget, Deadline: time.Now().Add(service.deliverTime())}
	return service.deliverWithin(domainId, records, budget)
}

func (service *PushService) RunPending(domainId string, budget JobBudget) (JobProgress, error) {
	records, failure := service.store.FindPendingPushes(domainId, budget.Writes)
	progress, problem := service.deliverWithin(domainId, records, budget)
	return service.withMore(progress, service.full(len(records), budget.Writes)), cmp.Or(failure, problem)
}

func (service *PushService) deliverWithin(domainId string, records []NotificationRecord, budget JobBudget) (JobProgress, error) {
	targets, keysMissing := service.targets.ForPlatforms()
	presences, presenceFailure := service.presenceUnless(keysMissing, domainId, records)
	return service.deliverAll(records, targets, presences, service.budgetAfter(presenceFailure, budget))
}

func (service *PushService) presenceUnless(keysMissing error, domainId string, records []NotificationRecord) (map[string]Presence, error) {
	if keysMissing != nil {
		return map[string]Presence{}, nil
	}
	return service.presence.States(domainId, service.memberIds(records))
}

func (service *PushService) memberIds(records []NotificationRecord) []string {
	memberIds := []string{}
	for _, record := range records {
		memberIds = append(memberIds, record.MemberId)
	}
	slices.Sort(memberIds)
	return slices.Compact(memberIds)
}

func (service *PushService) budgetAfter(presenceFailure error, budget JobBudget) JobBudget {
	if presenceFailure != nil {
		return JobBudget{Deadline: budget.Deadline}
	}
	return budget
}

func (service *PushService) deliverAll(records []NotificationRecord, targets map[string]webpush.Target, presences map[string]Presence, budget JobBudget) (JobProgress, error) {
	progress := JobProgress{}
	failures := []error{}
	for _, record := range records {
		step, failure := service.handle(record, targets, presences, service.remaining(budget, progress))
		progress, failures = service.added(progress, step), append(failures, failure)
	}
	return progress, cmp.Or(failures...)
}

func (service *PushService) handle(record NotificationRecord, targets map[string]webpush.Target, presences map[string]Presence, remaining JobBudget) (JobProgress, error) {
	if remaining.Writes < 1 || time.Now().After(remaining.Deadline) {
		return JobProgress{More: true}, nil
	}
	return service.push(record, targets, presences[record.MemberId], remaining.Writes)
}

func (service *PushService) push(record NotificationRecord, targets map[string]webpush.Target, presence Presence, writeLimit int) (JobProgress, error) {
	decision := notify.PushRules{}.Decide(presence.State, presence.DoNotDisturbUntil, time.Now())
	return service.byDecision(record, service.withTargets(decision, targets), targets, writeLimit)
}

func (service *PushService) withTargets(decision string, targets map[string]webpush.Target) string {
	if len(targets) == 0 {
		return service.notConfigured()
	}
	return decision
}

func (service *PushService) byDecision(record NotificationRecord, decision string, targets map[string]webpush.Target, writeLimit int) (JobProgress, error) {
	if decision != (notify.PushDecisions{}).Send() {
		return service.skip(record)
	}
	return service.send(record, targets, writeLimit)
}

func (service *PushService) skip(record NotificationRecord) (JobProgress, error) {
	failure := service.store.PatchPush(record, service.states.PushSkipped(), record.PushAttempts)
	return JobProgress{Written: 1, Skipped: 1, Done: 1}, failure
}

func (service *PushService) send(record NotificationRecord, targets map[string]webpush.Target, writeLimit int) (JobProgress, error) {
	result, failure := service.sender.Send(record, targets, writeLimit-1)
	outcome := service.strongest(result.Outcomes)
	attempts := record.PushAttempts + service.extraAttempt(outcome)
	state := service.stateFor(outcome, attempts)
	failure = service.patchUnless(failure, record, state, attempts)
	return service.progressFor(state, result.Written), failure
}

func (service *PushService) strongest(outcomes []string) string {
	ranked := service.ranked()
	index := slices.IndexFunc(ranked, func(outcome string) bool { return slices.Contains(outcomes, outcome) })
	return service.rankedAt(ranked, index)
}

func (service *PushService) rankedAt(ranked []string, index int) string {
	if index < 0 {
		return ""
	}
	return ranked[index]
}

func (service *PushService) ranked() []string {
	outcomes := webpush.Outcomes{}
	return []string{outcomes.Delivered(), outcomes.Retry(), outcomes.Refused(), outcomes.Gone()}
}

func (service *PushService) extraAttempt(outcome string) int {
	if outcome == (webpush.Outcomes{}).Retry() {
		return 1
	}
	return 0
}

func (service *PushService) stateFor(outcome string, attempts int) string {
	outcomes := webpush.Outcomes{}
	byOutcome := map[string]string{
		outcomes.Delivered(): service.states.PushSent(),
		outcomes.Retry():     service.retryState(attempts),
		outcomes.Refused():   service.states.PushFailed(),
	}
	return service.orSkipped(byOutcome, outcome)
}

func (service *PushService) retryState(attempts int) string {
	if attempts >= service.attemptLimit() {
		return service.states.PushFailed()
	}
	return service.states.PushPending()
}

func (service *PushService) orSkipped(byOutcome map[string]string, outcome string) string {
	state, found := byOutcome[outcome]
	if !found {
		return service.states.PushSkipped()
	}
	return state
}

func (service *PushService) patchUnless(failure error, record NotificationRecord, state string, attempts int) error {
	if failure != nil {
		return failure
	}
	return service.store.PatchPush(record, state, attempts)
}

func (service *PushService) progressFor(state string, removed int) JobProgress {
	byState := map[string]JobProgress{
		service.states.PushSent():    {Pushed: 1},
		service.states.PushPending(): {More: true},
	}
	progress := service.countedAs(byState, state)
	progress.Written, progress.Done = removed+1, 1
	return progress
}

func (service *PushService) countedAs(byState map[string]JobProgress, state string) JobProgress {
	progress, found := byState[state]
	if !found {
		return JobProgress{Skipped: 1}
	}
	return progress
}

func (service *PushService) remaining(budget JobBudget, progress JobProgress) JobBudget {
	return JobBudget{Writes: budget.Writes - progress.Written, Deadline: budget.Deadline}
}

func (service *PushService) added(total JobProgress, step JobProgress) JobProgress {
	return JobProgress{
		Written: total.Written + step.Written,
		Pushed:  total.Pushed + step.Pushed,
		Skipped: total.Skipped + step.Skipped,
		Done:    total.Done + step.Done,
		More:    total.More || step.More,
	}
}

func (service *PushService) full(waiting int, limit int) bool {
	return waiting > 0 && waiting >= limit
}

func (service *PushService) withMore(progress JobProgress, more bool) JobProgress {
	progress.More = progress.More || more
	return progress
}

func (service *PushService) notConfigured() string {
	return "skip_not_configured"
}

func (service *PushService) attemptLimit() int {
	return 5
}

func (service *PushService) deliverTime() time.Duration {
	return 5 * time.Second
}
