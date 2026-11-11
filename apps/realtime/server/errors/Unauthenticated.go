type Unauthenticated struct{}

func (failure *Unauthenticated) Error() string {
	return "the access token is missing, expired or not valid"
}
