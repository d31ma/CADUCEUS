package attachments

type Record struct {
	ID         string `json:"id,omitempty"`
	DomainId   string `json:"domainId"`
	UploaderId string `json:"uploaderId"`
	FileName   string `json:"fileName"`
	MediaType  string `json:"mediaType"`
	Size       int64  `json:"size"`
	Sha256     string `json:"sha256"`
	State      string `json:"state"`
	BlobKey    string `json:"blobKey"`
	CreatedAt  string `json:"createdAt"`
}
