type MemberDocument struct {
	Id             string   `json:"id,omitempty"`
	MemberId       string   `json:"memberId"`
	DomainId       string   `json:"domainId"`
	DisplayName    string   `json:"displayName"`
	PrimaryAddress string   `json:"primaryAddress"`
	State          string   `json:"state"`
	PendingEvents  []string `json:"pendingEvents"`
	DeadEvents     []string `json:"deadEvents"`
	OutboxPending  int      `json:"outboxPending"`
	OutboxDead     int      `json:"outboxDead"`
	OutboxWaiting  bool     `json:"outboxWaiting"`
	OutboxHasDead  bool     `json:"outboxHasDead"`
}
