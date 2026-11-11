package compose

import "time"

type Message struct {
	From        Address
	To          []Address
	Cc          []Address
	ReplyTo     []Address
	Subject     string
	MessageId   string
	InReplyTo   string
	References  []string
	Date        time.Time
	TextBody    string
	HtmlBody    string
	Attachments []Attachment
}
