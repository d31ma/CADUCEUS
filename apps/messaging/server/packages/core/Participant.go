package core

type Participant struct {
	Id             string `json:"id,omitempty"`
	DomainId       string `json:"domainId"`
	ConversationId string `json:"conversationId"`
	Kind           string `json:"kind"`
	MemberId       string `json:"memberId"`
	Address        string `json:"address"`
	DisplayName    string `json:"displayName"`
	Role           string `json:"role"`
	ReadSequence   int    `json:"readSequence"`
	ReadMessageId  string `json:"readMessageId"`
	ReadAt         string `json:"readAt"`
	OutboxFields
}
