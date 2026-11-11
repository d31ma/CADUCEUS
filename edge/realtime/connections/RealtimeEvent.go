package connections

type RealtimeEvent struct {
	Type           string `json:"type"`
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`
	State          string `json:"state"`
}
