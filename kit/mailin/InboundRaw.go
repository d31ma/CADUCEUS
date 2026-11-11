package mailin

import "mesail.dev/packages/outbox"

type InboundRaw struct {
	ID            string      `json:"id,omitempty"`
	DomainId      string      `json:"domainId"`
	ReceivedAt    string      `json:"receivedAt"`
	EnvelopeFrom  string      `json:"envelopeFrom"`
	Recipients    []string    `json:"rcpts"`
	RawBlobKey    string      `json:"rawBlobKey"`
	Size          int64       `json:"size"`
	RemoteAddress string      `json:"remoteAddress"`
	Helo          string      `json:"helo"`
	Tls           bool        `json:"tls"`
	MessageId     string      `json:"messageId"`
	Subject       string      `json:"subject"`
	FromAddress   string      `json:"fromAddress"`
	AuthResults   AuthResults `json:"authResults"`
	State         string      `json:"state"`
	EventQueued   bool        `json:"eventQueued"`
	PendingEvents []string    `json:"pendingEvents"`
	DeadEvents    []string    `json:"deadEvents"`
	OutboxPending int         `json:"outboxPending"`
	OutboxDead    int         `json:"outboxDead"`
	OutboxWaiting bool        `json:"outboxWaiting"`
	OutboxHasDead bool        `json:"outboxHasDead"`
}

func (raw InboundRaw) WireId() string {
	return "in_" + raw.ID
}

func (raw InboundRaw) OutboxFields() outbox.Fields {
	return outbox.Fields{PendingEvents: raw.PendingEvents, DeadEvents: raw.DeadEvents, OutboxPending: raw.OutboxPending, OutboxDead: raw.OutboxDead}
}
