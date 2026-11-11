type Duplicate struct {
	Message string
}

func (failure *Duplicate) Error() string {
	return failure.Message
}
