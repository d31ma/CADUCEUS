type MessageEmailSummary struct {
	Subject     string             `json:"subject"`
	From        string             `json:"from"`
	To          []string           `json:"to"`
	Cc          []string           `json:"cc"`
	Suspicious  bool               `json:"suspicious"`
	AuthResults MessageAuthResults `json:"authResults"`
}
