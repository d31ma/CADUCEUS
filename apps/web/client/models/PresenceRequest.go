type PresenceRequest struct {
	State             string `json:"state"`
	DoNotDisturbUntil string `json:"doNotDisturbUntil,omitempty"`
}
