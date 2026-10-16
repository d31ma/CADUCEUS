// A draft as GET, POST and PUT /drafts answer it. A stale PUT answers
// Conflict.
type Draft struct {
	QuoteProof    *QuoteProof         `json:"quoteProof,omitempty"`
	ID            string              `json:"id"`
	Sender        string              `json:"sender"`
	To            []string            `json:"to"`
	Cc            []string            `json:"cc"`
	Bcc           []string            `json:"bcc"`
	ReplyTo       []string            `json:"replyTo"`
	Subject       string              `json:"subject"`
	Body          string              `json:"body"`
	HTMLBody      string              `json:"htmlBody"`
	InReplyTo     string              `json:"inReplyTo"`
	References    []string            `json:"references"`
	SourceEmailID string              `json:"sourceEmailId"`
	Attachments   []ComposeAttachment `json:"attachments"`
	DraftClientID string              `json:"draftClientId"`
	DraftVersion  int                 `json:"draftVersion"`
	Recovered     bool                `json:"recovered"`
	Conflict      bool                `json:"conflict"`
}
