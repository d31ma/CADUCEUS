package events

type MessageEdited struct {
	MessageId      string   `json:"messageId"`
	ConversationId string   `json:"conversationId"`
	Body           string   `json:"body"`
	MemberIds      []string `json:"memberIds"`
	EditedAt       string   `json:"editedAt"`
}
