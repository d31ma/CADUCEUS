type ConversationOpened struct {
	DomainId       string `json:"domainId"`
	ConversationId string `json:"conversationId"`
	Title          string `json:"title"`
	Section        string `json:"section"`
	UnreadCount    int    `json:"unreadCount"`
}
