type Unauthenticated struct {
	Message string
}

func (failure *Unauthenticated) Error() string {
	return failure.Message
}

func (failure *Unauthenticated) Number() int {
	return 4012
}
