type OutboxFieldsPatch struct {
	PendingEvents []string `json:"pendingEvents"`
	DeadEvents    []string `json:"deadEvents"`
	OutboxPending int      `json:"outboxPending"`
	OutboxDead    int      `json:"outboxDead"`
	OutboxState   string   `json:"outboxState"`
	DeadState     string   `json:"deadState"`
}
