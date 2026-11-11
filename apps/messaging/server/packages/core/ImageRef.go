package core

type ImageRef struct {
	Alt          string `json:"alt"`
	AttachmentId string `json:"attachmentId,omitempty"`
	RemoteUrl    string `json:"remoteUrl,omitempty"`
}
