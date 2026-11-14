type BoxFull struct {
	Message     string
	ErrorNumber int
}

func (failure *BoxFull) Error() string {
	return failure.Message
}

func (failure *BoxFull) Number() int {
	return failure.ErrorNumber
}
