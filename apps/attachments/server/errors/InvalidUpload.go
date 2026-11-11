type InvalidUpload struct {
	Field  string
	Reason string
}

func (failure *InvalidUpload) Error() string {
	return failure.Field + ": " + failure.Reason
}
