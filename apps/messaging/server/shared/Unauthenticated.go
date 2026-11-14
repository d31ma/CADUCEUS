type Unauthenticated struct {
	Message     string
	ErrorNumber int
}

func (Unauthenticated) TokenRefused() *Unauthenticated {
	return &Unauthenticated{Message: "the access token was refused", ErrorNumber: 1501}
}

func (failure *Unauthenticated) Error() string {
	return failure.Message
}

func (failure *Unauthenticated) Number() int {
	return failure.ErrorNumber
}
