type SendRequested struct {
	MessageId      string   `json:"messageId"`
	From           string   `json:"from"`
	Rcpt           string   `json:"rcpt"`
	Role           string   `json:"role"`
	To             []string `json:"to"`
	Cc             []string `json:"cc"`
	Subject        string   `json:"subject"`
	InReplyTo      string   `json:"inReplyTo"`
	References     []string `json:"references"`
	AttachmentIds  []string `json:"attachmentIds"`
	IdempotencyKey string   `json:"idempotencyKey"`
}
