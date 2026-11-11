type InvalidRequest struct {
	Field  string
	Reason string
}

func (failure *InvalidRequest) Error() string {
	return failure.Field + ": " + failure.Reason
}
