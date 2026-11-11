package realtime

type Event struct {
	Type           string `json:"type"`
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`
	MemberId       string `json:"memberId,omitempty"`
	State          string `json:"state"`
}
