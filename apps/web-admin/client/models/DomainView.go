type DomainView struct {
	StateText    string      `json:"stateText"`
	StateColor   string      `json:"stateColor"`
	StateVariant string      `json:"stateVariant"`
	StateIcon    string      `json:"stateIcon"`
	Pending      bool        `json:"pending"`
	Waiting      string      `json:"waiting"`
	Status       string      `json:"status"`
	Records      []DnsRecord `json:"records"`
}
