type LocalPartDocument struct {
	Id        string   `json:"id,omitempty"`
	LocalPart string   `json:"localPart"`
	Owner     string   `json:"owner"`
	PersonId  string   `json:"personId"`
	DomainIds []string `json:"domainIds"`
}
