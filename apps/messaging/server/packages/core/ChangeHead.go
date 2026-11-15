package core

type ChangeHead struct {
	Id             string `json:"id,omitempty"`
	DomainId       string `json:"domainId"`
	ConversationId string `json:"conversationId"`
	LastChangeId   string `json:"lastChangeId"`
}
