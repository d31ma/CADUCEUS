// The body of POST /send and PUT /send/scheduled/{id}. A nil RemindAt or
// ReplyTo is sent as null, which clears it on a scheduled message.
type SendInput struct {
	From            string              `json:"from"`
	To              []string            `json:"to"`
	Cc              []string            `json:"cc"`
	Bcc             []string            `json:"bcc"`
	ReplyTo         []string            `json:"replyTo"`
	InReplyTo       string              `json:"inReplyTo,omitempty"`
	References      []string            `json:"references,omitempty"`
	Subject         string              `json:"subject"`
	HTML            string              `json:"html,omitempty"`
	Text            string              `json:"text"`
	Attachments     []ComposeAttachment `json:"attachments,omitempty"`
	OpenTracking    bool                `json:"openTracking"`
	LinkTracking    bool                `json:"linkTracking"`
	RemindAt        *string             `json:"remindAt"`
	Archive         bool                `json:"archive,omitempty"`
	ArchiveSourceID string              `json:"archiveSourceId,omitempty"`
	DelayMs         int                 `json:"delayMs,omitempty"`
	SendAt          string              `json:"sendAt,omitempty"`
	IdempotencyKey  string              `json:"idempotencyKey"`
}
