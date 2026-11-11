type DeadEvent struct {
	Collection string `json:"collection"`
	DocumentId string `json:"documentId"`
	EventId    string `json:"eventId"`
	Type       string `json:"type"`
	Subscriber string `json:"subscriber"`
	Attempts   int    `json:"attempts"`
	LastError  string `json:"lastError"`
	DeadAt     string `json:"deadAt"`
}
