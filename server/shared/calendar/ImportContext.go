// ImportContext is one delivered message to scan for invitations: the
// calendar owner it was delivered to, the stored email's id, its plain-text
// body and its attachments.
type ImportContext struct {
	Recipient     string
	SourceEmailId string
	TextBody      string
	Attachments   []ParsedAttachment
}
