package core

type NotFound struct {
	Message     string
	ErrorNumber int
}

func (NotFound) NoConversation() *NotFound {
	return &NotFound{Message: "no such conversation", ErrorNumber: 1538}
}

func (NotFound) NoMessage() *NotFound {
	return &NotFound{Message: "no such message", ErrorNumber: 1539}
}

func (NotFound) NoAttachment() *NotFound {
	return &NotFound{Message: "no such attachment", ErrorNumber: 1540}
}

func (NotFound) NoInboundMessage() *NotFound {
	return &NotFound{Message: "no such inbound message", ErrorNumber: 1541}
}

func (NotFound) NoOutboxDocument() *NotFound {
	return &NotFound{Message: "no such outbox document", ErrorNumber: 1542}
}

func (NotFound) NoOutboxCollection() *NotFound {
	return &NotFound{Message: "no such outbox collection", ErrorNumber: 1543}
}

func (NotFound) NoDeadEvent() *NotFound {
	return &NotFound{Message: "no such dead event", ErrorNumber: 1544}
}

func (NotFound) Kit(message string, number int) *NotFound {
	return &NotFound{Message: message, ErrorNumber: number}
}

func (failure *NotFound) Error() string {
	return failure.Message
}

func (failure *NotFound) Number() int {
	return failure.ErrorNumber
}
