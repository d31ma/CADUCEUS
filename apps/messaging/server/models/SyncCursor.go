type SyncCursor struct {
	ConversationId string `json:"conversationId"`
	After          string `json:"after,omitempty"`
}
