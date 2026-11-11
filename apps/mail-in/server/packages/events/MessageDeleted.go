package events

type MessageDeleted struct {
	MessageId      string   `json:"messageId"`
	ConversationId string   `json:"conversationId"`
	MemberIds      []string `json:"memberIds"`
	DeletedAt      string   `json:"deletedAt"`
}
