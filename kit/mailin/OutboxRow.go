package mailin

import "mesail.dev/packages/outbox"

type OutboxRow struct {
	Id            string   `json:"id"`
	PendingEvents []string `json:"pendingEvents"`
	DeadEvents    []string `json:"deadEvents"`
	OutboxPending int      `json:"outboxPending"`
	OutboxDead    int      `json:"outboxDead"`
}

func (row OutboxRow) Fields() outbox.Fields {
	return outbox.Fields{PendingEvents: row.PendingEvents, DeadEvents: row.DeadEvents, OutboxPending: row.OutboxPending, OutboxDead: row.OutboxDead}
}

func (row OutboxRow) IsWaiting() bool {
	return row.OutboxPending > 0
}

func (row OutboxRow) IsDead() bool {
	return row.OutboxDead > 0
}
