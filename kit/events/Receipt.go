package events

type Receipt struct {
	EventID string `json:"eventId"`
	Status  string `json:"status"`
}
