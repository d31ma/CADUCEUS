type ComposeAttachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int    `json:"size"`
	Disposition string `json:"disposition"`
	ContentID   string `json:"contentId"`
}
