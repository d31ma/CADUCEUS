package outbox

type runOutcome struct {
	updated     map[string]PendingEvent
	deadLetters []DeadLetter
	report      Report
}
