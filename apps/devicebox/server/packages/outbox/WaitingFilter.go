package outbox

type WaitingFilter struct {
	OutboxWaiting bool `json:"outboxWaiting"`
}
