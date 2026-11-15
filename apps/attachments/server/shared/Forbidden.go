import "cmp"

type Forbidden struct {
	Message     string
	ErrorNumber int
}

func (failure *Forbidden) Error() string {
	return failure.Message
}

func (failure *Forbidden) Number() int {
	return cmp.Or(failure.ErrorNumber, 3506)
}
