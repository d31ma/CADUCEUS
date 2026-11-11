package mailin

type Headers struct {
	MessageId  string    `json:"messageId"`
	InReplyTo  string    `json:"inReplyTo"`
	References []string  `json:"references"`
	Subject    string    `json:"subject"`
	From       Mailbox   `json:"from"`
	To         []Mailbox `json:"to"`
	Cc         []Mailbox `json:"cc"`
}
