package mailout

type Domain struct {
	DomainId     string `json:"domainId"`
	Name         string `json:"name"`
	State        string `json:"state"`
	DkimSelector string `json:"dkimSelector"`
}

func (Domain) activeState() string { return "active" }

func (domain Domain) IsActive() bool {
	return domain.State == domain.activeState()
}
