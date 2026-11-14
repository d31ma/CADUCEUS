package devicebox

type Forbidden struct {
	Message     string
	ErrorNumber int
}

func (Forbidden) NotADomainMember() *Forbidden {
	return &Forbidden{Message: "Only members of this domain can use the mailbox.", ErrorNumber: 6002}
}

func (failure *Forbidden) Error() string {
	return failure.Message
}

func (failure *Forbidden) Number() int {
	return failure.ErrorNumber
}
