type Notification struct {
	NotificationId string `json:"notificationId"`
	Kind           string `json:"kind"`
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`
	CreatedAt      string `json:"createdAt"`
	ReadAt         string `json:"readAt,omitempty"`
}
