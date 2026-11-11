package core

type MessageSource struct {
	MessageId     string   `json:"messageId"`
	DomainId      string   `json:"domainId"`
	Body          string   `json:"body"`
	AttachmentIds []string `json:"attachmentIds"`
}
