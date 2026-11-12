type UnexpectedFailure struct {
	ErrorCase
}

func (UnexpectedFailure) Unexpected() *UnexpectedFailure {
	return &UnexpectedFailure{ErrorCase{500, "unavailable", "", "Something went wrong."}}
}
