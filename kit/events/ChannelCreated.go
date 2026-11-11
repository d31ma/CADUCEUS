package events

type ChannelCreated struct {
	ChannelId      string `json:"channelId"`
	ConversationId string `json:"conversationId"`
	Name           string `json:"name"`
	Topic          string `json:"topic"`
	Visibility     string `json:"visibility"`
	EmailAddress   string `json:"emailAddress"`
	CreatedBy      string `json:"createdBy"`
}
