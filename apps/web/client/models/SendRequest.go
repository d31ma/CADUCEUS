type SendRequest struct {
	ClientMessageId string            `json:"clientMessageId"`
	ConversationId  string            `json:"conversationId,omitempty"`
	ThreadRootId    string            `json:"threadRootId,omitempty"`
	Body            string            `json:"body"`
	AttachmentIds   []string          `json:"attachmentIds"`
	Recipients      []RecipientChoice `json:"recipients"`
	Email           *EmailOptions     `json:"email,omitempty"`
}
