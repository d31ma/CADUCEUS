type SharedMessage struct {
	From            string             `json:"from"`
	To              []string           `json:"to"`
	Cc              []string           `json:"cc"`
	Subject         string             `json:"subject"`
	Body            string             `json:"body"`
	HTMLBody        string             `json:"htmlBody"`
	ReceivedAt      string             `json:"receivedAt"`
	Attachments     []SharedAttachment `json:"attachments"`
	Initial         string             `json:"initial"`
	ToLabel         string             `json:"toLabel"`
	CcLabel         string             `json:"ccLabel"`
	ReceivedAtLabel string             `json:"receivedAtLabel"`
	ProtectedHTML   string             `json:"protectedHtml"`
}
