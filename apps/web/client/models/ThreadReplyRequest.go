type ThreadReplyRequest struct {
	ClientMessageId string   `json:"clientMessageId"`
	ConversationId  string   `json:"conversationId"`
	ThreadRootId    string   `json:"threadRootId"`
	Body            string   `json:"body"`
	AttachmentIds   []string `json:"attachmentIds"`
}
