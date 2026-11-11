package attachments

type ReaderCheck interface {
	MayRead(attachmentId string, authorization string) bool
}
