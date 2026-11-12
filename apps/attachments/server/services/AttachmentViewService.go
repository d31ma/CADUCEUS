import "mesail.dev/packages/attachments"

// @Service
type AttachmentViewService struct {
}

func (service *AttachmentViewService) Attachment(view attachments.View) Attachment {
	return Attachment{AttachmentId: view.AttachmentId, FileName: view.FileName, MediaType: view.MediaType, Size: view.Size, Sha256: view.Sha256, State: view.State}
}
