type ThreadLoadRequested struct {
	ConversationId string `json:"conversationId"`
	RootId         string `json:"rootId"`
	Cursor         string `json:"cursor"`
}
