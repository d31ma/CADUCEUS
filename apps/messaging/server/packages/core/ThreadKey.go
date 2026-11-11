package core

type ThreadKey struct {
	Id             string `json:"id,omitempty"`
	DomainId       string `json:"domainId"`
	Key            string `json:"key"`
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`
}
