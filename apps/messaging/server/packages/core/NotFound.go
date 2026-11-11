package core

type NotFound struct {
	Message string
}

func (failure *NotFound) Error() string {
	return failure.Message
}
