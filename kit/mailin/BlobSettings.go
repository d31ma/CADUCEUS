package mailin

type BlobSettings struct {
	Adapter  string       `json:"adapter"`
	Folder   string       `json:"folder"`
	Bucket   string       `json:"bucket"`
	Region   string       `json:"region"`
	Endpoint string       `json:"endpoint"`
	Public   string       `json:"public"`
	Link     LinkSettings `json:"link"`
}
