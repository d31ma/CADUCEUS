type Unavailable struct {
	Message     string
	ErrorNumber int
}

func (Unavailable) Unexpected() *Unavailable {
	return &Unavailable{Message: "Something went wrong.", ErrorNumber: 5000}
}

func (Unavailable) RouteFailed() *Unavailable {
	return &Unavailable{Message: "Realtime is not available right now. Try again later.", ErrorNumber: 5024}
}

func (failure *Unavailable) Error() string {
	return "realtime is not available"
}

func (failure *Unavailable) Number() int {
	return failure.ErrorNumber
}
