package attachments

type UploadRequest struct {
	FileName  string
	MediaType string
	Size      int64
	Sha256    string
}
