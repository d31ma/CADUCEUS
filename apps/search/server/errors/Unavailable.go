type Unavailable struct {
	Message string
}

func (failure *Unavailable) Error() string {
	return failure.Message
}
