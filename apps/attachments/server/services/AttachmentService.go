import "mesail.dev/packages/attachments"

// @Service
type AttachmentService struct {
	// @Needs
	desks *DeskService
	// @Needs
	failures *FailureService
	// @Needs
	views *AttachmentViewService
}

func (service *AttachmentService) Create(authorization string, request CreateUploadRequest) (Upload, error) {
	terms := attachments.UploadRequest{FileName: request.FileName, MediaType: request.MediaType, Size: request.Size, Sha256: request.Sha256}
	upload, failed := service.desks.Desk().Create(authorization, terms)
	return Upload{AttachmentId: upload.AttachmentId, UploadUrl: upload.UploadUrl, ExpiresAt: upload.ExpiresAt}, service.failures.Translated(failed)
}

func (service *AttachmentService) Complete(authorization string, attachmentId string) (Attachment, error) {
	view, failed := service.desks.Desk().Complete(authorization, attachmentId)
	return service.views.Attachment(view), service.failures.Translated(failed)
}

func (service *AttachmentService) Download(authorization string, attachmentId string) (Download, error) {
	download, failed := service.desks.Desk().Download(authorization, attachmentId)
	return Download{Url: download.Url, ExpiresAt: download.ExpiresAt}, service.failures.Translated(failed)
}
