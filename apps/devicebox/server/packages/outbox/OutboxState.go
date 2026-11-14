package outbox

type OutboxState struct {
	Pending []PendingEvent
	Dead    []DeadLetter
}
