type ScheduledRow struct {
	ID             string `json:"id"`
	Subject        string `json:"subject"`
	Text           string `json:"text"`
	RecipientLabel string `json:"recipientLabel"`
	SendAtLabel    string `json:"sendAtLabel"`
	StatusLabel    string `json:"statusLabel"`
	LastError      string `json:"lastError"`
}
