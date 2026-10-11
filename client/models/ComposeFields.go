// ComposeRequested asks for the composer, prefilled. The shell's compose()
// stores the prefill in sessionStorage and opens /compose.
type ComposeRequested struct {
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
