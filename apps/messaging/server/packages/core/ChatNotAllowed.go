package core

type ChatNotAllowed struct {
	Address     string
	ErrorNumber int
}

func (ChatNotAllowed) For(address string) *ChatNotAllowed {
	return &ChatNotAllowed{Address: address, ErrorNumber: 1550}
}

func (failure *ChatNotAllowed) Error() string {
	return "chat is not allowed for " + failure.Address
}

func (failure *ChatNotAllowed) Number() int {
	return failure.ErrorNumber
}
