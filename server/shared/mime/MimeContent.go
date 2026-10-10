import "strings"

// MimeContent is a parsed message: its top-level headers, the text of its
// text/plain and text/html parts, and its attachments in order.
type MimeContent struct {
	Headers     MimeHeaders
	TextParts   []string
	HtmlParts   []string
	Attachments []MimeAttachment
}

// Text answers the text/plain parts joined by line breaks.
func (message MimeContent) Text() string {
	return strings.Join(message.TextParts, "\n")
}

// Html answers the text/html parts joined by line breaks, and whether the
// message has any.
func (message MimeContent) Html() (string, bool) {
	return strings.Join(message.HtmlParts, "\n"), len(message.HtmlParts) > 0
}
