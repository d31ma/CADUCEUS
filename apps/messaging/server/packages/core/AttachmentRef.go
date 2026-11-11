package core

type AttachmentRef struct {
	AttachmentId string `json:"attachmentId"`
	FileName     string `json:"fileName"`
	MediaType    string `json:"mediaType"`
	Size         int64  `json:"size"`
}
