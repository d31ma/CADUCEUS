type Forbidden struct {
	Message     string
	ErrorNumber int
}

func (failure *Forbidden) Error() string {
	return failure.Message
}

func (failure *Forbidden) Number() int {
	return failure.ErrorNumber
}
