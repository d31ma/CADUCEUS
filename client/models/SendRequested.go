type SendRequested struct {
	Mode           string          `json:"mode"`
	Envelope       MessageEnvelope `json:"envelope"`
	DraftID        string          `json:"draftId"`
	DraftClientID  string          `json:"draftClientId"`
	ScheduledID    string          `json:"scheduledId"`
	SendAt         string          `json:"sendAt"`
	DelayMs        int             `json:"delayMs"`
	IdempotencyKey string          `json:"idempotencyKey"`
}
