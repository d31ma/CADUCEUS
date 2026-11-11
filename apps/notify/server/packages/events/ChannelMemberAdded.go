package events

type ChannelMemberAdded struct {
	ChannelId      string `json:"channelId"`
	ConversationId string `json:"conversationId"`
	MemberId       string `json:"memberId"`
	JoinedAt       string `json:"joinedAt"`
}
