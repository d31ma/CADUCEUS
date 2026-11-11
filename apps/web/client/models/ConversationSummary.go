type ConversationSummary struct {
	ConversationId string `json:"conversationId"`
	Kind           string `json:"kind"`
	Title          string `json:"title"`
	HasExternal    bool   `json:"hasExternal"`
	UnreadCount    int    `json:"unreadCount"`
	MentionCount   int    `json:"mentionCount"`
	LastMessageAt  string `json:"lastMessageAt"`
}
