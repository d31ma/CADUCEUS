type ActivityEventRow struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	MessageID      string `json:"messageId"`
	EventIcon      string `json:"eventIcon"`
	EventLabel     string `json:"eventLabel"`
	SubjectLabel   string `json:"subjectLabel"`
	RecipientLabel string `json:"recipientLabel"`
	TargetLabel    string `json:"targetLabel"`
	Timestamp      string `json:"timestamp"`
	TimestampLabel string `json:"timestampLabel"`
}
