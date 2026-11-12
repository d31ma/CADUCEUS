type InvalidEnvelope struct {
	Field       string
	Reason      string
	ErrorNumber int
}

func (failure *InvalidEnvelope) Error() string {
	return failure.Field + ": " + failure.Reason
}

func (failure *InvalidEnvelope) Number() int {
	return failure.ErrorNumber
}
