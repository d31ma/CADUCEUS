type PendingDocument struct {
	Collection    string `json:"collection"`
	Id            string `json:"id"`
	OutboxPending int    `json:"outboxPending"`
	OutboxDead    int    `json:"outboxDead"`
}
