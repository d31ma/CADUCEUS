package events

type InvalidEnvelope struct {
	Reason string
}

func (failure InvalidEnvelope) Error() string {
	return "the event envelope is not valid: " + failure.Reason
}
