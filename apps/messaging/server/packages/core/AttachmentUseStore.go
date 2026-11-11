package core

type AttachmentUseStore interface {
	Insert(use AttachmentUse) (string, error)
	ByAttachment(domainId string, attachmentId string) ([]AttachmentUse, error)
}
