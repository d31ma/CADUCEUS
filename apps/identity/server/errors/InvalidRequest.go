type InvalidRequest struct{}

func (failure *InvalidRequest) Error() string {
	return "the domain id is neither a domain name nor *"
}
