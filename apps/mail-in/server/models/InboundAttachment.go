type InboundAttachment struct {
	FileName  string `json:"fileName"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	BlobKey   string `json:"blobKey"`
	ContentId string `json:"contentId,omitempty"`
}
