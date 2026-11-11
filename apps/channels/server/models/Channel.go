type Channel struct {
	ChannelId      string `json:"channelId"`
	Name           string `json:"name"`
	Topic          string `json:"topic,omitempty"`
	Visibility     string `json:"visibility"`
	EmailAddress   string `json:"emailAddress,omitempty"`
	Archived       bool   `json:"archived"`
	ConversationId string `json:"conversationId"`
}
