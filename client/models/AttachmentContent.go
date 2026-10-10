type AttachmentContent struct {
	Id            string  `json:"id"`
	Filename      string  `json:"filename"`
	ContentType   string  `json:"contentType"`
	ContentBase64 string  `json:"contentBase64"`
	Size          float64 `json:"size"`
}
