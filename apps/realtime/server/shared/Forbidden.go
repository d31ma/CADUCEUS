type Forbidden struct {
	ErrorNumber int
}

func (Forbidden) EventCallerNotAService(number int) *Forbidden {
	return &Forbidden{ErrorNumber: number}
}

func (failure *Forbidden) Error() string {
	return "this caller may not use this route"
}

func (failure *Forbidden) Number() int {
	return failure.ErrorNumber
}
