// One attachment of the message being written. ContentBase64 holds the bytes;
// the browser transport caps a request at 64 KiB, so attachments stay small.
type ComposeAttachment struct {
	ID            string `json:"id"`
	Filename      string `json:"filename"`
	ContentType   string `json:"contentType"`
	Size          int    `json:"size"`
	Disposition   string `json:"disposition"`
	ContentID     string `json:"contentId,omitempty"`
	ContentBase64 string `json:"contentBase64"`
}
