package core

type Reaction struct {
	Id             string `json:"id,omitempty"`
	DomainId       string `json:"domainId"`
	Key            string `json:"key"`
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`
	MemberId       string `json:"memberId"`
	Emoji          string `json:"emoji"`
	Active         bool   `json:"active"`
	OutboxFields
}
