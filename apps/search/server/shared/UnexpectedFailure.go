type UnexpectedFailure struct {
}

func (failure *UnexpectedFailure) Error() string {
	return "an unexpected error"
}

func (failure *UnexpectedFailure) Number() int {
	return 4000
}
