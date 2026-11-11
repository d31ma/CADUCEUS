type Unavailable struct{}

func (failure *Unavailable) Error() string {
	return "realtime is not available"
}
