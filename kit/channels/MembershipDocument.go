package channels

type MembershipDocument struct {
	Id            string   `json:"id,omitempty"`
	DomainId      string   `json:"domainId"`
	ChannelId     string   `json:"channelId"`
	MemberId      string   `json:"memberId"`
	JoinedAt      string   `json:"joinedAt"`
	Muted         bool     `json:"muted"`
	Left          bool     `json:"left"`
	PendingEvents []string `json:"pendingEvents"`
	DeadEvents    []string `json:"deadEvents"`
	OutboxPending int      `json:"outboxPending"`
	OutboxDead    int      `json:"outboxDead"`
	OutboxWaiting bool     `json:"outboxWaiting"`
	OutboxHasDead bool     `json:"outboxHasDead"`
}
