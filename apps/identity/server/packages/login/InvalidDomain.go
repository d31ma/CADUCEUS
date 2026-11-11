package login

type InvalidDomain struct{}

func (InvalidDomain) Error() string {
	return "the domain is neither a domain name nor *"
}
