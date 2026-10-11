// MimeAttachment is one attachment part of a parsed message, with its
// decoded bytes.
type MimeAttachment struct {
	Filename    string
	ContentType string
	Content     []byte
	Disposition string
	ContentId   string
}
