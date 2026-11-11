type FanoutRecord struct {
	Id         string   `json:"id,omitempty"`
	DomainId   string   `json:"domainId"`
	MessageId  string   `json:"messageId"`
	Message    string   `json:"message"`
	Recipients []string `json:"recipients"`
	State      string   `json:"state"`
	CreatedAt  string   `json:"createdAt"`
}
