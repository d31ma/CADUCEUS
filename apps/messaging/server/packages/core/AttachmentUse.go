package core

type AttachmentUse struct {
	Id             string `json:"id,omitempty"`
	DomainId       string `json:"domainId"`
	AttachmentId   string `json:"attachmentId"`
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`
}
