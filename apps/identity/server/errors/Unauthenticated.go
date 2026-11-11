type Unauthenticated struct{}

func (failure *Unauthenticated) Error() string {
	return "sign-in failed"
}
