type ForbiddenCaller struct {
	ErrorNumber int
}

func (failure *ForbiddenCaller) Error() string {
	return "only a caller with the Service role may deliver events"
}

func (failure *ForbiddenCaller) Number() int {
	return failure.ErrorNumber
}
