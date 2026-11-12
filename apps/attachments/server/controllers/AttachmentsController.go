// @RequestRoute("/api/v1/attachments")
// @SignedIn
type AttachmentsController struct {
	// @Needs
	attachments *AttachmentService
}

// @PostRoute
// @ResponseStatus(201)
func (controller *AttachmentsController) PostAttachment(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request CreateUploadRequest,
) (Upload, error) {
	return controller.attachments.Create(authorization, request)
}

// @PostRoute("/{attachment_id}/complete")
func (controller *AttachmentsController) PostComplete(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("attachment_id")
	attachmentId string,
) (Attachment, error) {
	return controller.attachments.Complete(authorization, attachmentId)
}

// @GetRoute("/{attachment_id}")
func (controller *AttachmentsController) GetAttachment(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("attachment_id")
	attachmentId string,
) (Download, error) {
	return controller.attachments.Download(authorization, attachmentId)
}
