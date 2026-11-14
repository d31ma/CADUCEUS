package devicebox

type Body struct {
	Id            string   `json:"id,omitempty"`
	EnvelopeId    string   `json:"envelopeId"`
	DomainId      string   `json:"domainId"`
	Ciphertext    string   `json:"ciphertext"`
	ExpiresUs     int64    `json:"expiresUs"`
	PendingEvents []string `json:"pendingEvents"`
	DeadEvents    []string `json:"deadEvents"`
	OutboxPending int      `json:"outboxPending"`
	OutboxDead    int      `json:"outboxDead"`
	OutboxWaiting bool     `json:"outboxWaiting"`
	OutboxHasDead bool     `json:"outboxHasDead"`
}
