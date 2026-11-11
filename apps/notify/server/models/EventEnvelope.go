import "encoding/json"

type EventEnvelope struct {
	EventId    string          `json:"eventId"`
	Type       string          `json:"type"`
	DomainId   string          `json:"domainId"`
	Producer   string          `json:"producer"`
	OccurredAt string          `json:"occurredAt"`
	Payload    json.RawMessage `json:"payload"`
}
