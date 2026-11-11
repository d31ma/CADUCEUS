// @RequestRoute("/api/v1/attachments")
// @SignedIn
type AttachmentsController struct {
	// @Needs
	attachmentCreateService *AttachmentCreateService
	// @Needs
	attachmentCompleteService *AttachmentCompleteService
	// @Needs
	attachmentDownloadService *AttachmentDownloadService
}

// @PostRoute
// @ResponseStatus(201)
func (controller *AttachmentsController) PostAttachment(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request CreateUploadRequest,
) (Upload, error) {
	return controller.attachmentCreateService.Create(authorization, request)
}

// @PostRoute("/{attachmentId}/complete")
func (controller *AttachmentsController) PostComplete(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	attachmentId string,
) (Attachment, error) {
	return controller.attachmentCompleteService.Complete(authorization, attachmentId)
}

// @GetRoute("/{attachmentId}")
func (controller *AttachmentsController) GetAttachment(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath
	attachmentId string,
) (Download, error) {
	return controller.attachmentDownloadService.Download(authorization, attachmentId)
}
