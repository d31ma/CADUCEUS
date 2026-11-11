type PersonRecord struct {
	PersonId        string   `json:"personId"`
	Username        string   `json:"username"`
	DisplayName     string   `json:"displayName"`
	DomainIds       []string `json:"domainIds"`
	AdminDomainIds  []string `json:"adminDomainIds"`
	DefaultDomainId string   `json:"defaultDomainId"`
	State           string   `json:"state"`
}
