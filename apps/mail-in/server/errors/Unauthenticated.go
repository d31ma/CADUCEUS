type Unauthenticated struct {
}

func (failure *Unauthenticated) Error() string {
	return "the access token was refused"
}
