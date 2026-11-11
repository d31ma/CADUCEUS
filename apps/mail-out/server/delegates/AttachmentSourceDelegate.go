import (
	"errors"

	"mesail.dev/packages/compose"
)

// @Delegate
// @NeedsAll()
type AttachmentSourceDelegate struct {
	blobs *BlobStoreDelegate
}

func (AttachmentSourceDelegate) contentType() string { return "application/octet-stream" }

func (AttachmentSourceDelegate) folder() string { return "/attachment-" }

func (source *AttachmentSourceDelegate) Attachments(domainId string, attachmentIds []string) ([]compose.Attachment, error) {
	attachments := []compose.Attachment{}
	var err error
	for _, attachmentId := range attachmentIds {
		content, readErr := source.blobs.Get(domainId, domainId+source.folder()+attachmentId)
		attachments = append(attachments, compose.Attachment{FileName: attachmentId, ContentType: source.contentType(), Content: content})
		err = errors.Join(err, readErr)
	}
	return attachments, err
}
