package channels

type Duplicate struct {
	Message     string
	ErrorNumber int
}

func (Duplicate) AddressTaken(message string) *Duplicate {
	return &Duplicate{Message: message, ErrorNumber: 2018}
}

func (failure *Duplicate) Error() string {
	return failure.Message
}

func (failure *Duplicate) Number() int {
	return failure.ErrorNumber
}
