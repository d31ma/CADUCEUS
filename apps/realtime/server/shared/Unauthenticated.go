type Unauthenticated struct {
	ErrorNumber int
}

func (failure *Unauthenticated) Error() string {
	return "the access token is missing, expired or not valid"
}

func (failure *Unauthenticated) Number() int {
	return failure.ErrorNumber
}
