package events

type ReadPositionMoved struct {
	ConversationId string `json:"conversationId"`
	MemberId       string `json:"memberId"`
	MessageId      string `json:"messageId"`
	ReadAt         string `json:"readAt"`
}
