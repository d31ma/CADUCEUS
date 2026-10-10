// ParsedAttachment is one attachment of a parsed or composed message. Its
// bytes are Content, or ContentBase64 when Content is nil.
type ParsedAttachment struct {
	Filename      string `json:"filename,omitempty"`
	ContentType   string `json:"contentType,omitempty"`
	Content       []byte `json:"-"`
	ContentBase64 string `json:"contentBase64,omitempty"`
	Disposition   string `json:"disposition,omitempty"`
	ContentId     string `json:"contentId,omitempty"`
}
