type MessageEnvelope struct {
	From          string   `json:"from"`
	To            []string `json:"to"`
	Cc            []string `json:"cc"`
	BlindCopy     []string `json:"bcc"`
	ReplyTo       []string `json:"replyTo"`
	Subject       string   `json:"subject"`
	InReplyTo     string   `json:"inReplyTo"`
	References    []string `json:"references"`
	SourceEmailID string   `json:"sourceEmailId"`
	OpenTracking  bool     `json:"openTracking"`
	LinkTracking  bool     `json:"linkTracking"`
	RemindAt      string   `json:"remindAt"`
}
