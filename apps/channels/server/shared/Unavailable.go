type Unavailable struct {
	Message     string
	ErrorNumber int
}

func (Unavailable) Unexpected() *Unavailable {
	return &Unavailable{Message: "Something went wrong.", ErrorNumber: 2000}
}

func (Unavailable) ChannelRouteFailed() *Unavailable {
	return &Unavailable{Message: "the channel service is not available", ErrorNumber: 2027}
}

func (Unavailable) OutboxRouteFailed() *Unavailable {
	return &Unavailable{Message: "the outbox is not available", ErrorNumber: 2028}
}

func (failure *Unavailable) Error() string {
	return failure.Message
}

func (failure *Unavailable) Number() int {
	return failure.ErrorNumber
}
