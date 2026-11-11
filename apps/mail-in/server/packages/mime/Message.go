package mime

type Message struct {
	Headers     Headers
	Text        string
	Html        string
	Attachments []Attachment
}
