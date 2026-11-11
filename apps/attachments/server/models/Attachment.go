type Attachment struct {
	AttachmentId string `json:"attachmentId"`
	FileName     string `json:"fileName"`
	MediaType    string `json:"mediaType"`
	Size         int64  `json:"size"`
	Sha256       string `json:"sha256"`
	State        string `json:"state"`
}
