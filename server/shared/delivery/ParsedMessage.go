// ParsedMessage is an inbound message body read for delivery: its text,
// HTML, attachments and lower-case headers (address headers as their
// comma-separated addresses).
type ParsedMessage struct {
	Text        string
	Html        string
	Attachments []ParsedAttachment
	Headers     map[string]any
}
