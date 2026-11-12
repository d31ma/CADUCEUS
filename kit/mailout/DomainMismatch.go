package mailout

type DomainMismatch struct {
	DomainId string
}

func (failure DomainMismatch) Error() string {
	return "the caller may not act for the domain " + failure.DomainId
}

func (failure DomainMismatch) Number() int {
	return 3003
}
