type SendRequest struct {
	ClientMessageId    string            `json:"clientMessageId"`
	ConversationId     string            `json:"conversationId"`
	ThreadRootId       string            `json:"threadRootId,omitempty"`
	AlsoToConversation bool              `json:"alsoToConversation"`
	Body               string            `json:"body"`
	AttachmentIds      []string          `json:"attachmentIds"`
	Recipients         []RecipientChoice `json:"recipients"`
	Email              *EmailOptions     `json:"email,omitempty"`
}
