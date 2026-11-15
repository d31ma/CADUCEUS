package core

type Change struct {
	Id             string `json:"id,omitempty"`
	DomainId       string `json:"domainId"`
	ConversationId string `json:"conversationId"`
	MessageId      string `json:"messageId"`
	EventId        string `json:"eventId"`
	Month          string `json:"month"`
	LoggedAt       string `json:"loggedAt"`
}
