package events

type InvalidEnvelope struct {
	Reason      string
	ErrorNumber int
}

func (InvalidEnvelope) EmptyEventId() InvalidEnvelope {
	return InvalidEnvelope{Reason: "eventId must not be empty", ErrorNumber: 1}
}

func (InvalidEnvelope) DomainTooLong() InvalidEnvelope {
	return InvalidEnvelope{Reason: "domainId is longer than 253 characters", ErrorNumber: 2}
}

func (InvalidEnvelope) DomainCharacters() InvalidEnvelope {
	return InvalidEnvelope{Reason: "domainId may only hold the characters a-z, 0-9, - and .", ErrorNumber: 3}
}

func (InvalidEnvelope) DomainWithoutDot() InvalidEnvelope {
	return InvalidEnvelope{Reason: "domainId must hold at least one dot", ErrorNumber: 4}
}

func (InvalidEnvelope) DomainEmptyLabel() InvalidEnvelope {
	return InvalidEnvelope{Reason: "domainId must not hold an empty label", ErrorNumber: 5}
}

func (InvalidEnvelope) UnknownType(cause UnknownEventType) InvalidEnvelope {
	return InvalidEnvelope{Reason: cause.Error(), ErrorNumber: cause.Number()}
}

func (InvalidEnvelope) WrongProducer(producer string, expected string) InvalidEnvelope {
	return InvalidEnvelope{Reason: "the producer " + producer + " does not produce this event type; " + expected + " does", ErrorNumber: 7}
}

func (InvalidEnvelope) OccurredAtNotATime() InvalidEnvelope {
	return InvalidEnvelope{Reason: "occurredAt must be an RFC 3339 time", ErrorNumber: 8}
}

func (InvalidEnvelope) EmptyPayload() InvalidEnvelope {
	return InvalidEnvelope{Reason: "payload must not be empty", ErrorNumber: 9}
}

func (InvalidEnvelope) DataAfterPayload() InvalidEnvelope {
	return InvalidEnvelope{Reason: "payload holds data after its JSON value", ErrorNumber: 10}
}

func (InvalidEnvelope) PayloadShape() InvalidEnvelope {
	return InvalidEnvelope{Reason: "the payload is not a JSON object of the expected shape", ErrorNumber: 11}
}

func (failure InvalidEnvelope) Error() string {
	return "the event envelope is not valid: " + failure.Reason
}

func (failure InvalidEnvelope) Number() int {
	return failure.ErrorNumber
}
