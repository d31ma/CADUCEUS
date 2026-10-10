type ScheduledSend struct {
	ID           string           `json:"id"`
	From         string           `json:"from"`
	To           any              `json:"to"`
	Cc           any              `json:"cc"`
	ReplyTo      any              `json:"replyTo"`
	Subject      string           `json:"subject"`
	Text         string           `json:"text"`
	HTML         string           `json:"html"`
	InReplyTo    string           `json:"inReplyTo"`
	References   []string         `json:"references"`
	OpenTracking bool             `json:"openTracking"`
	LinkTracking bool             `json:"linkTracking"`
	Attachments  []map[string]any `json:"attachments"`
	SendAt       string           `json:"sendAt"`
	Status       string           `json:"status"`
	Attempts     int              `json:"attempts"`
	NextAttempt  string           `json:"nextAttempt"`
	LastError    string           `json:"lastError"`
	RemindAt     string           `json:"remindAt"`
}
