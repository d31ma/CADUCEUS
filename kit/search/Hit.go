package search

type Hit struct {
	MessageId      string `json:"messageId"`
	ConversationId string `json:"conversationId"`
	Snippet        string `json:"snippet"`
	CreatedAt      string `json:"createdAt"`
}
