type Conflict struct {
	Message     string
	ErrorNumber int
}

func (failure *Conflict) Error() string {
	return failure.Message
}

func (failure *Conflict) Number() int {
	return failure.ErrorNumber
}
