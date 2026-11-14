package devicebox

type Unauthenticated struct {
	Message     string
	ErrorNumber int
}

func (Unauthenticated) TokenRefused() *Unauthenticated {
	return &Unauthenticated{Message: "Sign in again.", ErrorNumber: 6001}
}

func (failure *Unauthenticated) Error() string {
	return failure.Message
}

func (failure *Unauthenticated) Number() int {
	return failure.ErrorNumber
}
