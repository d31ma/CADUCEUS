package core

import (
	"mesail.dev/packages/outbox"
)

type OutboxFields struct {
	PendingEvents []string `json:"pendingEvents"`
	DeadEvents    []string `json:"deadEvents"`
	OutboxPending int      `json:"outboxPending"`
	OutboxDead    int      `json:"outboxDead"`
}

func (fields OutboxFields) Fields() outbox.Fields {
	return outbox.Fields{PendingEvents: fields.PendingEvents, DeadEvents: fields.DeadEvents, OutboxPending: fields.OutboxPending, OutboxDead: fields.OutboxDead}
}

func (OutboxFields) From(fields outbox.Fields) OutboxFields {
	return OutboxFields{PendingEvents: fields.PendingEvents, DeadEvents: fields.DeadEvents, OutboxPending: fields.OutboxPending, OutboxDead: fields.OutboxDead}
}
