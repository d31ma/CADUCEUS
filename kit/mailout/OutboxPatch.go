package mailout

import "mesail.dev/packages/outbox"

type OutboxPatch struct {
	PendingEvents []string `json:"pendingEvents"`
	DeadEvents    []string `json:"deadEvents"`
	OutboxPending int      `json:"outboxPending"`
	OutboxDead    int      `json:"outboxDead"`
	OutboxWaiting bool     `json:"outboxWaiting"`
	OutboxHasDead bool     `json:"outboxHasDead"`
}

func NewOutboxPatch(fields outbox.Fields) OutboxPatch {
	return OutboxPatch{PendingEvents: fields.PendingEvents, DeadEvents: fields.DeadEvents, OutboxPending: fields.OutboxPending, OutboxDead: fields.OutboxDead, OutboxWaiting: fields.OutboxPending > 0, OutboxHasDead: fields.OutboxDead > 0}
}
