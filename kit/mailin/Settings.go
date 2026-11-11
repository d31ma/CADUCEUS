package mailin

type Settings struct {
	Directory string       `json:"directory"`
	Blobs     BlobSettings `json:"blobs"`
}
