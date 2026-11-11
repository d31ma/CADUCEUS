package reception

type DomainNotHosted struct{}

func (DomainNotHosted) Error() string {
	return "the domain is not hosted here"
}
