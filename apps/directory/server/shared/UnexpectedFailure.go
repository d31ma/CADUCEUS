type UnexpectedFailure struct {
	ErrorCase
}

func (UnexpectedFailure) Unexpected() *UnexpectedFailure {
	return &UnexpectedFailure{ErrorCase{1000, "unavailable", "", "Something went wrong."}}
}
