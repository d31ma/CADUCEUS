package events

import "encoding/json"

type Envelope struct {
	EventID    string          `json:"eventId"`
	Type       string          `json:"type"`
	DomainId   string          `json:"domainId"`
	Producer   string          `json:"producer"`
	OccurredAt string          `json:"occurredAt"`
	Payload    json.RawMessage `json:"payload"`
}
