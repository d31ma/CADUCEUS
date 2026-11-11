type InboundEventPatch struct {
	PendingEvents []string `json:"pendingEvents"`
	EventQueued   bool     `json:"eventQueued"`
}
