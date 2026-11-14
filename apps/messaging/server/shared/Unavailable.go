type Unavailable struct {
	Message     string
	ErrorNumber int
}

func (Unavailable) Unexpected() *Unavailable {
	return &Unavailable{Message: "Something went wrong.", ErrorNumber: 1500}
}

func (Unavailable) ServiceFailed() *Unavailable {
	return &Unavailable{Message: "the messaging service is not available", ErrorNumber: 1507}
}

func (failure *Unavailable) Error() string {
	return failure.Message
}

func (failure *Unavailable) Number() int {
	return failure.ErrorNumber
}
