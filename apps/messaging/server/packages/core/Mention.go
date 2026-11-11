package core

type Mention struct {
	Id             string `json:"id,omitempty"`
	DomainId       string `json:"domainId"`
	ConversationId string `json:"conversationId"`
	MemberId       string `json:"memberId"`
	MessageId      string `json:"messageId"`
	Sequence       int    `json:"sequence"`
}
