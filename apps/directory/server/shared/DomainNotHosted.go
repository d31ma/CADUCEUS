type DomainNotHosted struct {
	Message string
}

func (failure *DomainNotHosted) Error() string {
	return failure.Message
}
