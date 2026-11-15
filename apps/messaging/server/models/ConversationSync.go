type ConversationSync struct {
	ConversationId string    `json:"conversationId"`
	State          string    `json:"state"`
	Messages       []Message `json:"messages"`
	Next           string    `json:"next"`
	More           bool      `json:"more"`
}
