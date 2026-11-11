type UnknownDomain struct{}

func (failure *UnknownDomain) Error() string {
	return "not found"
}
