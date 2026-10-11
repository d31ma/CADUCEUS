// The complete autosave snapshot that POST and PUT /drafts take.
type DraftInput struct {
	ClientID      string              `json:"clientId"`
	Version       int                 `json:"version,omitempty"`
	From          string              `json:"from,omitempty"`
	To            []string            `json:"to"`
	Cc            []string            `json:"cc"`
	Bcc           []string            `json:"bcc"`
	ReplyTo       []string            `json:"replyTo"`
	Subject       string              `json:"subject"`
	Text          string              `json:"text"`
	HTML          string              `json:"html"`
	InReplyTo     string              `json:"inReplyTo,omitempty"`
	References    []string            `json:"references"`
	SourceEmailID string              `json:"sourceEmailId,omitempty"`
	Attachments   []ComposeAttachment `json:"attachments"`
}
