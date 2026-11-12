type Forbidden struct {
	Message string
}

func (failure *Forbidden) Error() string {
	return failure.Message
}

func (failure *Forbidden) Number() int {
	return 3506
}
