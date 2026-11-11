type NotificationRow struct {
	NotificationId string `json:"notificationId"`
	Kind           string `json:"kind"`
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`
	Title          string `json:"title"`
	Time           string `json:"time"`
	Stamp          string `json:"stamp"`
	Day            string `json:"day"`
	Unread         bool   `json:"unread"`
	MarkLabel      string `json:"markLabel"`
}
