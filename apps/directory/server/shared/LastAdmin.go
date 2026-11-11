type LastAdmin struct {
	Message string
}

func (failure *LastAdmin) Error() string {
	return failure.Message
}
