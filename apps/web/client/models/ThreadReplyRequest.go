type ThreadReplyRequest struct {
	ClientMessageId    string   `json:"clientMessageId"`
	ConversationId     string   `json:"conversationId"`
	ThreadRootId       string   `json:"threadRootId"`
	AlsoToConversation bool     `json:"alsoToConversation"`
	Body               string   `json:"body"`
	AttachmentIds      []string `json:"attachmentIds"`
}
