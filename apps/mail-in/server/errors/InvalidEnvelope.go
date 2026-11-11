type InvalidEnvelope struct {
	Field  string
	Reason string
}

func (failure *InvalidEnvelope) Error() string {
	return failure.Field + ": " + failure.Reason
}
