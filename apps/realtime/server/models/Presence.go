type Presence struct {
	MemberId          string `json:"memberId,omitempty"`
	State             string `json:"state"`
	DoNotDisturbUntil string `json:"doNotDisturbUntil,omitempty"`
	LastSeenAt        string `json:"lastSeenAt"`
}
