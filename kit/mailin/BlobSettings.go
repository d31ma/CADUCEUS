package mailin

type BlobSettings struct {
	Adapter string       `json:"adapter"`
	Folder  string       `json:"folder"`
	Bucket  string       `json:"bucket"`
	Link    LinkSettings `json:"link"`
}
