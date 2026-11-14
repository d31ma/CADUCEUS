type InvalidRequest struct {
	Message     string
	Field       string
	ErrorNumber int
}

func (failure *InvalidRequest) Error() string {
	return failure.Message
}

func (failure *InvalidRequest) Number() int {
	return failure.ErrorNumber
}
