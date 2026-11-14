// @RequestRoute("/api/v1/messaging/attachments")
// @SignedIn
type AttachmentsController struct {
	// @Needs
	attachments *AttachmentRouteService
}

// @GetRoute("/{attachment_id}/access")
// @ResponseStatus(204)
func (controller *AttachmentsController) GetAccess(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("attachment_id")
	attachmentId string,
) error {
	return controller.attachments.Access(authorization, attachmentId)
}
