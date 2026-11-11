type RetryRequest struct {
	Collection string `json:"collection"`
	DocumentId string `json:"documentId"`
	EventId    string `json:"eventId"`
	Subscriber string `json:"subscriber"`
}
