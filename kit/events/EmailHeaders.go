package events

type EmailHeaders struct {
	MessageId  string   `json:"messageId"`
	InReplyTo  string   `json:"inReplyTo"`
	References []string `json:"references"`
	Subject    string   `json:"subject"`
	From       string   `json:"from"`
	To         []string `json:"to"`
	Cc         []string `json:"cc"`
}
