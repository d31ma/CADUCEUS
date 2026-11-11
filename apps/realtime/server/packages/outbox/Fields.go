package outbox

type Fields struct {
	PendingEvents []string `json:"pendingEvents"`
	DeadEvents    []string `json:"deadEvents"`
	OutboxPending int      `json:"outboxPending"`
	OutboxDead    int      `json:"outboxDead"`
}
