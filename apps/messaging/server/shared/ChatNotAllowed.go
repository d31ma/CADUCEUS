type ChatNotAllowed struct {
	Message string
	Address string
}

func (failure *ChatNotAllowed) Error() string {
	return failure.Message
}
