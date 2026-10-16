// The composer fields that ComposeLaunched carries: a reply, a forward, a
// draft or a scheduled message to edit, or a manifest launch.
type ComposePrefill struct {
	// Quote is app-generated reply/forward provenance, separate from untrusted Body.
	Quote         *QuotedMessage      `json:"quote,omitempty"`
	DraftID       string              `json:"draftId"`
	ScheduledID   string              `json:"scheduledId"`
	SendAt        string              `json:"sendAt"`
	RemindAt      string              `json:"remindAt"`
	From          string              `json:"from"`
	To            string              `json:"to"`
	Cc            string              `json:"cc"`
	Bcc           string              `json:"bcc"`
	ReplyTo       string              `json:"replyTo"`
	Subject       string              `json:"subject"`
	Body          string              `json:"body"`
	InReplyTo     string              `json:"inReplyTo"`
	References    []string            `json:"references"`
	SourceEmailID string              `json:"sourceEmailId"`
	Attachments   []ComposeAttachment `json:"attachments"`
	OpenTracking  bool                `json:"openTracking"`
	LinkTracking  bool                `json:"linkTracking"`
}
