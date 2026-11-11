type PersonSummary struct {
	PersonId    string   `json:"personId"`
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	DomainIds   []string `json:"domainIds"`
	State       string   `json:"state"`
}
