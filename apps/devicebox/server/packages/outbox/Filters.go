package outbox

type Filters struct{}

func (Filters) Waiting() WaitingFilter {
	return WaitingFilter{OutboxWaiting: true}
}

func (Filters) HasDead() DeadFilter {
	return DeadFilter{OutboxHasDead: true}
}
