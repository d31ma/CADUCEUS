package outbox

type runOutcome struct {
	updated     map[string]PendingEvent
	deadLetters []DeadLetter
	report      Report
}

func (outcome runOutcome) attempted() bool {
	return outcome.report.Delivered+outcome.report.Retried+outcome.report.DeadLettered > 0
}
