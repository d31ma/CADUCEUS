type InvalidUpload struct {
	Field       string
	Reason      string
	ErrorNumber int
}

func (failure *InvalidUpload) Error() string {
	return failure.Field + ": " + failure.Reason
}

func (failure *InvalidUpload) Number() int {
	return failure.ErrorNumber
}
