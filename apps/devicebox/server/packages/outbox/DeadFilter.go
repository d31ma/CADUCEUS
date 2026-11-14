package outbox

type DeadFilter struct {
	OutboxHasDead bool `json:"outboxHasDead"`
}
