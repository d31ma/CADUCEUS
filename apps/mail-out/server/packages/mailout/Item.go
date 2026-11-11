package mailout

import "mesail.dev/packages/outbox"

type Item struct {
	Id             string   `json:"id,omitempty"`
	DomainId       string   `json:"domainId"`
	EventId        string   `json:"eventId"`
	Producer       string   `json:"producer"`
	MessageId      string   `json:"messageId"`
	From           string   `json:"from"`
	FromName       string   `json:"fromName"`
	Rcpt           string   `json:"rcpt"`
	Role           string   `json:"role"`
	To             []string `json:"to"`
	Cc             []string `json:"cc"`
	Subject        string   `json:"subject"`
	InReplyTo      string   `json:"inReplyTo"`
	References     []string `json:"references"`
	AttachmentIds  []string `json:"attachmentIds"`
	IdempotencyKey string   `json:"idempotencyKey"`
	RawBlobKey     string   `json:"rawBlobKey"`
	Size           int64    `json:"size"`
	Attempts       int      `json:"attempts"`
	QueuedAt       string   `json:"queuedAt"`
	NextAttemptAt  string   `json:"nextAttemptAt"`
	ExpiresAt      string   `json:"expiresAt"`
	State          string   `json:"state"`
	Detail         string   `json:"detail"`
	PendingEvents  []string `json:"pendingEvents"`
	DeadEvents     []string `json:"deadEvents"`
	OutboxPending  int      `json:"outboxPending"`
	OutboxDead     int      `json:"outboxDead"`
	OutboxWaiting  bool     `json:"outboxWaiting"`
	OutboxHasDead  bool     `json:"outboxHasDead"`
}

func (item Item) OutboxFields() outbox.Fields {
	return outbox.Fields{PendingEvents: item.PendingEvents, DeadEvents: item.DeadEvents, OutboxPending: item.OutboxPending, OutboxDead: item.OutboxDead}
}

func (item Item) WithOutbox(fields outbox.Fields) Item {
	item.PendingEvents, item.DeadEvents = fields.PendingEvents, fields.DeadEvents
	item.OutboxPending, item.OutboxDead = fields.OutboxPending, fields.OutboxDead
	item.OutboxWaiting, item.OutboxHasDead = fields.OutboxPending > 0, fields.OutboxDead > 0
	return item
}
