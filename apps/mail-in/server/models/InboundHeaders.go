type InboundHeaders struct {
	MessageId  string           `json:"messageId"`
	InReplyTo  string           `json:"inReplyTo"`
	References []string         `json:"references"`
	Subject    string           `json:"subject"`
	From       MailboxAddress   `json:"from"`
	To         []MailboxAddress `json:"to"`
	Cc         []MailboxAddress `json:"cc"`
}
