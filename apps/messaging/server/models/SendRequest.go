type SendRequest struct {
	ClientMessageId    string            `json:"clientMessageId"`
	ConversationId     string            `json:"conversationId,omitempty"`
	ThreadRootId       string            `json:"threadRootId,omitempty"`
	AlsoToConversation bool              `json:"alsoToConversation,omitempty"`
	Body               string            `json:"body,omitempty"`
	AttachmentIds      []string          `json:"attachmentIds,omitempty"`
	Recipients         []RecipientChoice `json:"recipients,omitempty"`
	Email              *EmailOptions     `json:"email,omitempty"`
}
