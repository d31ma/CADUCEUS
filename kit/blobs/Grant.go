package blobs

type Grant struct {
	Method    string
	Key       Key
	ExpiresAt int64
	MediaType string
	Size      int64
	FileName  string
}
