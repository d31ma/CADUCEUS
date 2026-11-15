package channels

type NotFound struct {
	Message     string
	ErrorNumber int
}

func (NotFound) BadChannelId() *NotFound {
	return &NotFound{Message: "no such channel", ErrorNumber: 2007}
}

func (NotFound) ChannelHidden() *NotFound {
	return &NotFound{Message: "no such channel", ErrorNumber: 2008}
}

func (NotFound) MemberOutsideDomain() *NotFound {
	return &NotFound{Message: "no such member in this domain", ErrorNumber: 2011}
}

func (NotFound) OutboxLetter(message string, number int) *NotFound {
	return &NotFound{Message: message, ErrorNumber: number}
}

func (NotFound) OutboxDocument() *NotFound {
	return &NotFound{Message: "no such outbox document", ErrorNumber: 2025}
}

func (NotFound) OutboxCollection() *NotFound {
	return &NotFound{Message: "no such outbox collection", ErrorNumber: 2026}
}

func (failure *NotFound) Error() string {
	return failure.Message
}

func (failure *NotFound) Number() int {
	return failure.ErrorNumber
}
