// @RequestRoute("/api/v1/attachments")
// @Allow(Service)
type AttachmentMetaController struct {
	// @Needs
	metas *AttachmentMetaService
}

// @GetRoute("/{attachment_id}/meta")
func (controller *AttachmentMetaController) GetMeta(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("attachment_id")
	attachmentId string,
) (Attachment, error) {
	return controller.metas.Meta(authorization, attachmentId)
}
