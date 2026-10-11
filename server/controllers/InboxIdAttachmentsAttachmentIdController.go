import "net/http"

// @RequestRoute("/inbox/{id}/attachments/{attachment_id}")
// @SignedIn
type InboxIdAttachmentsAttachmentIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	attachmentsHelper *AttachmentsHelper
	// @Needs
	emailsHelper *EmailsHelper
}

// @GetRoute
func (controller *InboxIdAttachmentsAttachmentIdController) GetAttachment(
	// @FromPath
	id string,
	// @FromPath
	attachment_id string,
	request *http.Request,
) *http.Response {
	return InboxAttachmentRoute{Parts: MailboxParts{
		Access: controller.accessHelper, Attachments: controller.attachmentsHelper, Emails: controller.emailsHelper,
	}}.Read(id, attachment_id, request)
}
