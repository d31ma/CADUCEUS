package storage

type SeenEvent struct {
	Id       string `json:"id,omitempty"`
	DomainId string `json:"domainId"`
	EventId  string `json:"eventId"`
}
