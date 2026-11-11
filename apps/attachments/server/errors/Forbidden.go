type Forbidden struct {
	Message string
}

func (failure *Forbidden) Error() string {
	return failure.Message
}
