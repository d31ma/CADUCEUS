package channels

type Conflict struct {
	Message string
}

func (failure *Conflict) Error() string {
	return failure.Message
}
