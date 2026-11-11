type SummaryRecord struct {
	Id              string   `json:"id,omitempty"`
	DomainId        string   `json:"domainId"`
	MemberId        string   `json:"memberId"`
	BatchKey        string   `json:"batchKey"`
	Rcpt            string   `json:"rcpt"`
	Subject         string   `json:"subject"`
	Body            string   `json:"body"`
	NotificationIds []string `json:"notificationIds"`
	CreatedAt       string   `json:"createdAt"`
	PendingEvents   []string `json:"pendingEvents"`
	DeadEvents      []string `json:"deadEvents"`
	OutboxPending   int      `json:"outboxPending"`
	OutboxDead      int      `json:"outboxDead"`
	OutboxState     string   `json:"outboxState"`
	DeadState       string   `json:"deadState"`
}
