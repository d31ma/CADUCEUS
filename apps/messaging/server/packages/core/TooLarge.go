package core

type TooLarge struct {
	Message     string
	ErrorNumber int
}

func (TooLarge) StoredMessage() *TooLarge {
	return &TooLarge{Message: "the message is too large to store; send a shorter body", ErrorNumber: 1548}
}

func (TooLarge) AttachmentTotal() *TooLarge {
	return &TooLarge{Message: "the attachments of a message may have at most 50 MiB together", ErrorNumber: 1549}
}

func (failure *TooLarge) Error() string {
	return failure.Message
}

func (failure *TooLarge) Number() int {
	return failure.ErrorNumber
}
