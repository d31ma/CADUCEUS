type UploadCreateRequest struct {
	FileName  string `json:"fileName"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Sha256    string `json:"sha256"`
}
