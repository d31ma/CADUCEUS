type EmailSummary struct {
	Subject     string      `json:"subject"`
	From        string      `json:"from"`
	To          []string    `json:"to"`
	Cc          []string    `json:"cc"`
	Suspicious  bool        `json:"suspicious"`
	AuthResults AuthResults `json:"authResults"`
}
