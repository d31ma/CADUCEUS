package realtime

type ChannelMember struct {
	ID             string `json:"id,omitempty"`
	DomainId       string `json:"domainId"`
	ConversationId string `json:"conversationId"`
	MemberId       string `json:"memberId"`
}
