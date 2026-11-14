type TooLarge struct {
	Message     string
	ErrorNumber int
}

func (failure *TooLarge) Error() string {
	return failure.Message
}

func (failure *TooLarge) Number() int {
	return failure.ErrorNumber
}
