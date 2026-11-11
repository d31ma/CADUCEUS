package core

type ChatNotAllowed struct {
	Address string
}

func (failure *ChatNotAllowed) Error() string {
	return "chat is not allowed for " + failure.Address
}
