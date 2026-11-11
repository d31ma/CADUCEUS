package events

type ChannelArchived struct {
	ChannelId      string `json:"channelId"`
	ConversationId string `json:"conversationId"`
}
