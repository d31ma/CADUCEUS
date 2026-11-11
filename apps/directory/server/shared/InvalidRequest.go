type InvalidRequest struct {
	Field   string
	Message string
}

func (failure *InvalidRequest) Error() string {
	return failure.Message
}
