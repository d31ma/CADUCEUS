package attachments

type View struct {
	AttachmentId string
	FileName     string
	MediaType    string
	Size         int64
	Sha256       string
	State        string
}
