// @Service
type AttachmentMetaService struct {
	// @Needs
	desks *DeskService
	// @Needs
	failures *FailureService
	// @Needs
	views *AttachmentViewService
}

func (service *AttachmentMetaService) Meta(authorization string, attachmentId string) (Attachment, error) {
	view, failed := service.desks.Desk().Meta(authorization, attachmentId)
	return service.views.Attachment(view), service.failures.Translated(failed)
}
