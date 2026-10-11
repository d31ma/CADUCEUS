type TrackingEvent struct {
	Type           string `json:"type"`
	EventIcon      string `json:"eventIcon"`
	EventLabel     string `json:"eventLabel"`
	Timestamp      string `json:"timestamp"`
	TimestampLabel string `json:"timestampLabel"`
	RecipientLabel string `json:"recipientLabel"`
	TargetLabel    string `json:"targetLabel"`
}
