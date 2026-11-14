package realtime

type Forbidden struct {
	ErrorNumber int
}

func (Forbidden) NotAService() Forbidden {
	return Forbidden{ErrorNumber: 5017}
}

func (Forbidden) Error() string {
	return "this caller may not use this route"
}

func (failure Forbidden) Number() int {
	return failure.ErrorNumber
}
