type Unavailable struct {
	Message string
}

func (failure *Unavailable) Error() string {
	return failure.Message
}

func (failure *Unavailable) Number() int {
	return 4015
}
