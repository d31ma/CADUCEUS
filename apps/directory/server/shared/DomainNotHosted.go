type DomainNotHosted struct {
	ErrorCase
}

func (DomainNotHosted) Address() *DomainNotHosted {
	return &DomainNotHosted{ErrorCase{1014, "domain_not_hosted", "", "this domain is not hosted here"}}
}
