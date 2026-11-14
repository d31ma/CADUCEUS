package outbox

import "time"

type documentRun struct {
	pusher      Pusher
	policy      RetryPolicy
	subscribers map[string]Subscriber
	now         time.Time
	blocked     map[string]bool
	outcome     runOutcome
}

func (run *documentRun) deliver(pending []PendingEvent) runOutcome {
	for _, event := range pending {
		run.deliverEvent(event)
	}
	return run.outcome
}

func (run *documentRun) deliverEvent(event PendingEvent) {
	kinds := attemptKind{}
	groups := newAttemptGroups(run.attemptAll(event, run.isDue(event)))
	run.block(groups.names(kinds.retry()))
	run.block(groups.names(kinds.skipped()))
	run.buryFailed(event, groups.of(kinds.dead()))
	event.Waiting = groups.remaining()
	next := run.reschedule(event, groups)
	keep, doomed := run.splitExhausted(next)
	run.buryWaiting(next, doomed)
	next.Waiting = keep
	run.outcome.updated[next.Envelope.EventID] = next
	run.outcome.report = run.outcome.report.plus(Report{
		Delivered:    groups.count(kinds.delivered()),
		Retried:      groups.count(kinds.retry()),
		DeadLettered: groups.count(kinds.dead()) + len(doomed),
	})
}

func (run *documentRun) isDue(event PendingEvent) bool {
	nextAttempt, _ := time.Parse(time.RFC3339Nano, event.NextAttemptAt)
	return !nextAttempt.After(run.now)
}

func (run *documentRun) attemptAll(event PendingEvent, due bool) []pushAttempt {
	attempts := []pushAttempt{}
	for _, name := range event.Waiting {
		attempts = append(attempts, run.attempt(event, name, due))
	}
	return attempts
}

func (run *documentRun) attempt(event PendingEvent, name string, due bool) pushAttempt {
	if !run.isOpen(name, due) {
		return pushAttempt{subscriber: name, skipped: true}
	}
	subscriber := Subscriber{Name: name, URL: run.subscribers[name].URL}
	return pushAttempt{subscriber: name, err: run.pusher.Push(subscriber, event.Envelope)}
}

func (run *documentRun) isOpen(name string, due bool) bool {
	return due && !run.blocked[name]
}

func (run *documentRun) block(names []string) {
	for _, name := range names {
		run.blocked[name] = true
	}
}

func (run *documentRun) reschedule(event PendingEvent, groups attemptGroups) PendingEvent {
	if groups.count(attemptKind{}.retry()) == 0 {
		return event
	}
	return run.failedAgain(event, groups.lastError())
}

func (run *documentRun) failedAgain(event PendingEvent, lastError string) PendingEvent {
	event.Attempts = event.Attempts + 1
	event.NextAttemptAt = run.stamp(run.now.Add(run.policy.Delay(event.Attempts)))
	event.LastError = lastError
	return event
}

func (run *documentRun) splitExhausted(event PendingEvent) ([]string, []string) {
	if !run.policy.Exhausted(event.Attempts) {
		return event.Waiting, []string{}
	}
	return []string{}, event.Waiting
}

func (run *documentRun) buryFailed(event PendingEvent, failed []pushAttempt) {
	for _, attempt := range failed {
		run.bury(run.deadLetter(event, attempt.subscriber, event.Attempts+1, attempt.err.Error()))
	}
}

func (run *documentRun) buryWaiting(event PendingEvent, names []string) {
	for _, name := range names {
		run.bury(run.deadLetter(event, name, event.Attempts, event.LastError))
	}
}

func (run *documentRun) bury(letter DeadLetter) {
	run.outcome.deadLetters = append(run.outcome.deadLetters, letter)
}

func (run *documentRun) deadLetter(event PendingEvent, subscriber string, attempts int, lastError string) DeadLetter {
	return DeadLetter{Envelope: event.Envelope, Subscriber: subscriber, Attempts: attempts, LastError: lastError, DeadAt: run.stamp(run.now)}
}

func (run *documentRun) stamp(moment time.Time) string {
	return moment.UTC().Format(time.RFC3339Nano)
}
