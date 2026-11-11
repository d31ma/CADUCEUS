package core

type TooLarge struct {
	Message string
}

func (failure *TooLarge) Error() string {
	return failure.Message
}
