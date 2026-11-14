type Forbidden struct {
	ErrorNumber int
}

func (Forbidden) EventCallerNotAService() *Forbidden {
	return &Forbidden{ErrorNumber: 5018}
}

func (failure *Forbidden) Error() string {
	return "this caller may not use this route"
}

func (failure *Forbidden) Number() int {
	return failure.ErrorNumber
}
