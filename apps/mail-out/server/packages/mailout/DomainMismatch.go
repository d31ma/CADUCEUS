package mailout

type DomainMismatch struct {
	DomainId string
}

func (failure DomainMismatch) Error() string {
	return "the caller may not act for the domain " + failure.DomainId
}
