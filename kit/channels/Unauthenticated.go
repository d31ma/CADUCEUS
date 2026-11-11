package channels

type Unauthenticated struct {
	Message string
}

func (failure *Unauthenticated) Error() string {
	return failure.Message
}
