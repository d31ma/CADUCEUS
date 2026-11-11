package webpush

type Message struct {
	Title          string `json:"title"`
	Body           string `json:"body"`
	Tag            string `json:"tag"`
	Url            string `json:"url"`
	NotificationId string `json:"notificationId"`
	Kind           string `json:"kind"`
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`
}
