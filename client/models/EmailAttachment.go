type EmailAttachment struct {
	Id           string  `json:"id"`
	Filename     string  `json:"filename"`
	Size         float64 `json:"size"`
	ContentType  string  `json:"contentType"`
	ContentId    string  `json:"contentId"`
	Disposition  string  `json:"disposition"`
	PreviewKind  string  `json:"previewKind"`
	ThumbnailUrl string  `json:"thumbnailUrl"`
	SizeLabel    string  `json:"sizeLabel"`
}
