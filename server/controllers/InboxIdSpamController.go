import "net/http"

// @RequestRoute("/inbox/{id}/spam")
// @SignedIn
type InboxIdSpamController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	attachmentsHelper *AttachmentsHelper
	// @Needs
	categoryMappingService *CategoryMappingService
	// @Needs
	emailsHelper *EmailsHelper
}

// @PutRoute
func (controller *InboxIdSpamController) PutSpam(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return InboxSpamRoute{Parts: MailboxParts{
		Access: controller.accessHelper, Attachments: controller.attachmentsHelper,
		Categories: controller.categoryMappingService, Emails: controller.emailsHelper,
	}}.Report(id, request)
}
