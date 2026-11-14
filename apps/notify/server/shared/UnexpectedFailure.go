type UnexpectedFailure struct {
	ErrorCase
}

func (UnexpectedFailure) Unexpected() *UnexpectedFailure {
	return &UnexpectedFailure{ErrorCase{4500, "unavailable", "", "Something went wrong."}}
}
