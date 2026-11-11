type SearchHitRow struct {
	MessageId      string          `json:"messageId"`
	ConversationId string          `json:"conversationId"`
	CreatedAt      string          `json:"createdAt"`
	TimeLabel      string          `json:"timeLabel"`
	Parts          []SearchHitPart `json:"parts"`
}
