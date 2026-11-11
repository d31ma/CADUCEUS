type DeadLetterView struct {
	EventId    string `json:"eventId"`
	Type       string `json:"type"`
	DomainId   string `json:"domainId"`
	Subscriber string `json:"subscriber"`
	Attempts   int    `json:"attempts"`
	LastError  string `json:"lastError"`
	DeadAt     string `json:"deadAt"`
}
