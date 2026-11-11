package mime

type Attachment struct {
	FileName  string
	MediaType string
	ContentId string
	Inline    bool
	Size      int64
	Content   []byte
}
