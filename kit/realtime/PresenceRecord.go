package realtime

type PresenceRecord struct {
	ID                string `json:"id,omitempty"`
	DomainId          string `json:"domainId"`
	MemberId          string `json:"memberId"`
	State             string `json:"state"`
	DoNotDisturbUntil string `json:"doNotDisturbUntil"`
	LastSeenAt        string `json:"lastSeenAt"`
	DisconnectedAt    string `json:"disconnectedAt"`
}
