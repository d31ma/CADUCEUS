type PersonRow struct {
	PersonId       string   `json:"personId"`
	Name           string   `json:"name"`
	Username       string   `json:"username"`
	Initials       string   `json:"initials"`
	DomainIds      []string `json:"domainIds"`
	State          string   `json:"state"`
	StateText      string   `json:"stateText"`
	StateColor     string   `json:"stateColor"`
	NoDomains      bool     `json:"noDomains"`
	Deleted        bool     `json:"deleted"`
	Deleting       bool     `json:"deleting"`
	Disabled       bool     `json:"disabled"`
	AccountManaged bool     `json:"accountManaged"`
	MenuId         string   `json:"menuId"`
	ActionsLabel   string   `json:"actionsLabel"`
	AccountLabel   string   `json:"accountLabel"`
	DomainsLabel   string   `json:"domainsLabel"`
}
