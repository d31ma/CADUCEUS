type UploadItem struct {
	FileName     string `json:"fileName"`
	MediaType    string `json:"mediaType"`
	Size         int64  `json:"size"`
	Sha256       string `json:"sha256"`
	SizeText     string `json:"sizeText"`
	State        string `json:"state"`
	AttachmentId string `json:"attachmentId"`
	Problem      string `json:"problem"`
}
