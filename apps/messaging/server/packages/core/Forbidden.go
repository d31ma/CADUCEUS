package core

type Forbidden struct {
	Message     string
	ErrorNumber int
}

func (Forbidden) NotTheAuthor() *Forbidden {
	return &Forbidden{Message: "only the author can change this message", ErrorNumber: 1537}
}

func (failure *Forbidden) Error() string {
	return failure.Message
}

func (failure *Forbidden) Number() int {
	return failure.ErrorNumber
}
