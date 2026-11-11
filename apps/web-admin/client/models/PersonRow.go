type PersonRow struct {
	PersonId       string   `json:"personId"`
	Name           string   `json:"name"`
	Username       string   `json:"username"`
	Initials       string   `json:"initials"`
	DomainIds      []string `json:"domainIds"`
	State          string   `json:"state"`
	StateText      string   `json:"stateText"`
	Deleted        bool     `json:"deleted"`
	Disabled       bool     `json:"disabled"`
	AccountManaged bool     `json:"accountManaged"`
}
