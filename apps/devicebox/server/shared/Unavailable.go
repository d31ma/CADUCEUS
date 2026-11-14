type Unavailable struct {
	Message     string
	ErrorNumber int
}

func (Unavailable) Unexpected() *Unavailable {
	return &Unavailable{Message: "Something went wrong.", ErrorNumber: 6000}
}

func (Unavailable) MailboxRouteFailed() *Unavailable {
	return &Unavailable{Message: "The mailbox is not available.", ErrorNumber: 6028}
}

func (Unavailable) OutboxRouteFailed() *Unavailable {
	return &Unavailable{Message: "The outbox is not available.", ErrorNumber: 6029}
}

func (failure *Unavailable) Error() string {
	return failure.Message
}

func (failure *Unavailable) Number() int {
	return failure.ErrorNumber
}
