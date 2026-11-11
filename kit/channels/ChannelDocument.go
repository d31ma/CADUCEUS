package channels

type ChannelDocument struct {
	Id             string   `json:"id,omitempty"`
	DomainId       string   `json:"domainId"`
	Name           string   `json:"name"`
	Topic          string   `json:"topic"`
	Visibility     string   `json:"visibility"`
	EmailAddress   string   `json:"emailAddress"`
	Archived       bool     `json:"archived"`
	ConversationId string   `json:"conversationId"`
	CreatedBy      string   `json:"createdBy"`
	CreatedAt      string   `json:"createdAt"`
	PendingEvents  []string `json:"pendingEvents"`
	DeadEvents     []string `json:"deadEvents"`
	OutboxPending  int      `json:"outboxPending"`
	OutboxDead     int      `json:"outboxDead"`
	OutboxWaiting  bool     `json:"outboxWaiting"`
	OutboxHasDead  bool     `json:"outboxHasDead"`
}
