type ActivityEvent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	MessageID string `json:"messageId"`
	Subject   string `json:"subject"`
	Recipient string `json:"recipient"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Timestamp string `json:"timestamp"`
}
