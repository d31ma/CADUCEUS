package core

type AttachmentInfo struct {
	AttachmentId string `json:"attachmentId"`
	FileName     string `json:"fileName"`
	MediaType    string `json:"mediaType"`
	Size         int64  `json:"size"`
	State        string `json:"state"`
}
