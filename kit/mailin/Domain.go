package mailin

type Domain struct {
	DomainId string `json:"domainId"`
	Name     string `json:"name"`
	State    string `json:"state"`
}

func (domain Domain) IsActive() bool {
	return domain.State == domain.activeState()
}

func (Domain) activeState() string {
	return "active"
}
