package outbox

type answeredPushes struct {
	byKey map[string]pushAttempt
}

func newAnsweredPushes() answeredPushes {
	return answeredPushes{byKey: map[string]pushAttempt{}}
}

func (answers answeredPushes) push(event PendingEvent, name string) pushAttempt {
	attempt, answered := answers.byKey[answers.key(event.Envelope.EventID, name)]
	if !answered {
		return pushAttempt{subscriber: name, skipped: true}
	}
	return attempt
}

func (answers answeredPushes) take(message laneMessage) int {
	if message.finished {
		return 1
	}
	answers.byKey[answers.key(message.eventId, message.attempt.subscriber)] = message.attempt
	return 0
}

func (answeredPushes) key(eventId string, name string) string {
	return eventId + "\x00" + name
}
