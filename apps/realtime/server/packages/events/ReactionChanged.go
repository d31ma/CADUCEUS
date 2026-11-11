package events

type ReactionChanged struct {
	MessageId      string   `json:"messageId"`
	ConversationId string   `json:"conversationId"`
	MemberId       string   `json:"memberId"`
	Emoji          string   `json:"emoji"`
	Added          bool     `json:"added"`
	MemberIds      []string `json:"memberIds"`
}
