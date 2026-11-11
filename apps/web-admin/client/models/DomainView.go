type DomainView struct {
	StateText  string      `json:"stateText"`
	StateClass string      `json:"stateClass"`
	StateIcon  string      `json:"stateIcon"`
	Pending    bool        `json:"pending"`
	Waiting    string      `json:"waiting"`
	Status     string      `json:"status"`
	Records    []DnsRecord `json:"records"`
}
