type ChangedMessage struct {
	Message     string
	Current     Message
	ErrorNumber int
}

func (ChangedMessage) Of(message string, current Message, number int) *ChangedMessage {
	return &ChangedMessage{Message: message, Current: current, ErrorNumber: number}
}

func (failure *ChangedMessage) Error() string {
	return failure.Message
}

func (failure *ChangedMessage) Number() int {
	return failure.ErrorNumber
}
