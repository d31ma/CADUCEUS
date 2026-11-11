package mime

type Headers struct {
	MessageId  string
	InReplyTo  string
	References []string
	Subject    string
	From       Mailbox
	To         []Mailbox
	Cc         []Mailbox
	Date       string
}
