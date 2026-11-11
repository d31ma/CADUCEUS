package realtime

type EventPayload struct {
	MessageId      string   `json:"messageId"`
	ConversationId string   `json:"conversationId"`
	ChannelId      string   `json:"channelId"`
	MemberId       string   `json:"memberId"`
	AuthorMemberId string   `json:"authorMemberId"`
	CreatedBy      string   `json:"createdBy"`
	Emoji          string   `json:"emoji"`
	State          string   `json:"state"`
	MemberIds      []string `json:"memberIds"`
}
