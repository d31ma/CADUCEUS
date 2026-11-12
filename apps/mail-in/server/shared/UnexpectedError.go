type UnexpectedError struct {
}

func (failure *UnexpectedError) Error() string {
	return "Something went wrong."
}

func (failure *UnexpectedError) Number() int {
	return 2500
}
