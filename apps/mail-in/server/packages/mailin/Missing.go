package mailin

type Missing struct {
	Message     string
	ErrorNumber int
}

func (Missing) NoOutboxCollection() Missing {
	return Missing{Message: "no such outbox collection", ErrorNumber: 2510}
}

func (Missing) NoInboundMessage() Missing {
	return Missing{Message: "no such inbound message", ErrorNumber: 2511}
}

func (Missing) NoDeadLetter(message string, number int) Missing {
	return Missing{Message: message, ErrorNumber: number}
}

func (Missing) NoOutboxDocument() Missing {
	return Missing{Message: "no such outbox document", ErrorNumber: 2513}
}

func (missing Missing) Error() string {
	return missing.Message
}

func (missing Missing) Number() int {
	return missing.ErrorNumber
}
