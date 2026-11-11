package core

type AttachmentCatalog interface {
	Describe(domainId string, attachmentId string) (AttachmentInfo, bool, error)
}
