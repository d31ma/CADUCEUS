type SeenEventRecord struct {
	Id       string `json:"id,omitempty"`
	DomainId string `json:"domainId"`
	EventId  string `json:"eventId"`
	SeenAt   string `json:"seenAt"`
}
