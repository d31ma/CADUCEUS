type NotFound struct {
	ErrorNumber int
}

func (failure *NotFound) Error() string {
	return "not found"
}

func (failure *NotFound) Number() int {
	return failure.ErrorNumber
}
