// ComposeFields is the composer prefill, stored as caduceus_compose_prefill
// in sessionStorage before /compose opens.
type ComposeFields struct {
	DraftID      string           `json:"draftId,omitempty"`
	ScheduledID  string           `json:"scheduledId,omitempty"`
	SendAt       string           `json:"sendAt,omitempty"`
	RemindAt     string           `json:"remindAt,omitempty"`
	From         string           `json:"from,omitempty"`
	To           string           `json:"to,omitempty"`
	Cc           string           `json:"cc,omitempty"`
	ReplyTo      string           `json:"replyTo,omitempty"`
	Subject      string           `json:"subject,omitempty"`
	Body         string           `json:"body,omitempty"`
	InReplyTo    string           `json:"inReplyTo,omitempty"`
	References   []string         `json:"references,omitempty"`
	Attachments  []map[string]any `json:"attachments,omitempty"`
	OpenTracking bool             `json:"openTracking,omitempty"`
	LinkTracking bool             `json:"linkTracking,omitempty"`
}
