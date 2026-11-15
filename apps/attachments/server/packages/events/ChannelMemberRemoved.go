package events

type ChannelMemberRemoved struct {
	ChannelId      string `json:"channelId"`
	ConversationId string `json:"conversationId"`
	MemberId       string `json:"memberId"`
}
