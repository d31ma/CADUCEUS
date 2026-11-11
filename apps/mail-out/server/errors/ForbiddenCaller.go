type ForbiddenCaller struct{}

func (failure *ForbiddenCaller) Error() string {
	return "only a caller with the Service role may deliver events"
}
