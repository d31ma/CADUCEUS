package backups

type SeenEvent struct {
	DomainId string `json:"domainId"`
	EventId  string `json:"eventId"`
}
