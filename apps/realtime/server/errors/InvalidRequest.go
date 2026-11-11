type InvalidRequest struct {
	Field  string
	Reason string
}

func (failure *InvalidRequest) Error() string {
	return "invalid request: " + failure.Field
}
