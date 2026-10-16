// MessageFields is the message being written, as the composer sends or
// autosaves it.
type MessageFields struct {
	QuoteProof    *QuoteProof
	From          string
	To            string
	Cc            string
	Bcc           string
	ReplyTo       string
	Subject       string
	HTML          string
	Text          string
	InReplyTo     string
	References    []string
	SourceEmailID string
	Attachments   []ComposeAttachment
	OpenTracking  bool
	LinkTracking  bool
	RemindAt      string
}
