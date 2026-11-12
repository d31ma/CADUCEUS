type UnknownDomain struct {
	ErrorNumber int
}

func (failure *UnknownDomain) Error() string {
	return "not found"
}

func (failure *UnknownDomain) Number() int {
	return failure.ErrorNumber
}
