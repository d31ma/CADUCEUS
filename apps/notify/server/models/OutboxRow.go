type OutboxRow struct {
	Id            string   `json:"id"`
	PendingEvents []string `json:"pendingEvents"`
	DeadEvents    []string `json:"deadEvents"`
	OutboxPending int      `json:"outboxPending"`
	OutboxDead    int      `json:"outboxDead"`
	OutboxWaiting bool     `json:"outboxWaiting"`
	OutboxHasDead bool     `json:"outboxHasDead"`
}
