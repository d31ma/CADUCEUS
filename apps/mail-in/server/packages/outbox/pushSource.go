package outbox

type pushSource interface {
	push(event PendingEvent, name string) pushAttempt
}
