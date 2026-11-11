package realtime

type SeenRecord struct {
	ID       string `json:"id,omitempty"`
	DomainId string `json:"domainId"`
	EventId  string `json:"eventId"`
}
