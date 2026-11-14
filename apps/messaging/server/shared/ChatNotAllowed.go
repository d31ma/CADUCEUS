type ChatNotAllowed struct {
	Message     string
	Address     string
	ErrorNumber int
}

func (failure *ChatNotAllowed) Error() string {
	return failure.Message
}

func (failure *ChatNotAllowed) Number() int {
	return failure.ErrorNumber
}
