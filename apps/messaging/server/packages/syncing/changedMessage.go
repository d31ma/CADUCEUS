package syncing

type changedMessage struct {
	MessageId      string `json:"messageId"`
	ConversationId string `json:"conversationId"`
	ThreadRootId   string `json:"threadRootId"`
}
