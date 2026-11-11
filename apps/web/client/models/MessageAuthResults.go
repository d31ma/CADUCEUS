type MessageAuthResults struct {
	SenderPolicy   string `json:"spf"`
	SignatureCheck string `json:"dkim"`
	DomainPolicy   string `json:"dmarc"`
	Handoff        bool   `json:"handoff,omitempty"`
}
