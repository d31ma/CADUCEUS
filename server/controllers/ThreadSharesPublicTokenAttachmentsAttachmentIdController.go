import "net/http"

// @RequestRoute("/thread-shares/public/{token}/attachments/{attachment_id}")
// @Public
type ThreadSharesPublicTokenAttachmentsAttachmentIdController struct {
	// @Needs
	threadSharingService *ThreadSharingService
}

// @GetRoute
func (controller *ThreadSharesPublicTokenAttachmentsAttachmentIdController) GetSharedAttachment(
	// @FromPath
	token string,
	// @FromPath
	attachment_id string,
) *http.Response {
	return ThreadSharesRoute{Sharing: controller.threadSharingService}.Download(token, attachment_id)
}
