package events

type PresenceChanged struct {
	MemberId          string `json:"memberId"`
	State             string `json:"state"`
	DoNotDisturbUntil string `json:"doNotDisturbUntil"`
	LastSeenAt        string `json:"lastSeenAt"`
}
