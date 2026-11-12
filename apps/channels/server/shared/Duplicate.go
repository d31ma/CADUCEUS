type Duplicate struct {
	Message     string
	ErrorNumber int
}

func (failure *Duplicate) Error() string {
	return failure.Message
}

func (failure *Duplicate) Number() int {
	return failure.ErrorNumber
}
