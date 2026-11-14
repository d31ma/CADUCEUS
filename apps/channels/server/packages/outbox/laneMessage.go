package outbox

type laneMessage struct {
	eventId  string
	attempt  pushAttempt
	finished bool
}
