package core

type Unavailable struct {
	Message string
	Cause   error
}

func (failure *Unavailable) Error() string {
	return failure.Message
}

func (failure *Unavailable) Unwrap() error {
	return failure.Cause
}
