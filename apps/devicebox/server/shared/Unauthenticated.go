type Unauthenticated struct {
	Message     string
	ErrorNumber int
}

func (failure *Unauthenticated) Error() string {
	return failure.Message
}

func (failure *Unauthenticated) Number() int {
	return failure.ErrorNumber
}
