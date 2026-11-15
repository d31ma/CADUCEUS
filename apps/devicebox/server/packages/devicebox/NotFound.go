package devicebox

type NotFound struct {
	Message     string
	ErrorNumber int
}

func (NotFound) SenderDevice() *NotFound {
	return &NotFound{Message: "The sending device is not one of your devices in this domain.", ErrorNumber: 6010}
}

func (NotFound) ReceivingDevice() *NotFound {
	return &NotFound{Message: "A receiving device is not a device of a member of this domain.", ErrorNumber: 6011}
}

func (NotFound) Box() *NotFound {
	return &NotFound{Message: "This device is not one of your devices in this domain.", ErrorNumber: 6014}
}

func (NotFound) OutboxLetter(message string, number int) *NotFound {
	return &NotFound{Message: message, ErrorNumber: number}
}

func (NotFound) OutboxDocument() *NotFound {
	return &NotFound{Message: "There is no such outbox document.", ErrorNumber: 6026}
}

func (NotFound) OutboxCollection() *NotFound {
	return &NotFound{Message: "There is no such outbox collection.", ErrorNumber: 6027}
}

func (failure *NotFound) Error() string {
	return failure.Message
}

func (failure *NotFound) Number() int {
	return failure.ErrorNumber
}
