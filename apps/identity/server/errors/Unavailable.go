type Unavailable struct{}

func (failure *Unavailable) Error() string {
	return "sign-in is not available now"
}
