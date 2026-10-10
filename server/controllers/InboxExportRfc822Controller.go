import "net/http"

// @RequestRoute("/inbox/export/rfc822")
// @SignedIn
type InboxExportRfc822Controller struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	categoryMappingService *CategoryMappingService
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	rawMessagesHelper *RawMessagesHelper
}

// @GetRoute
func (controller *InboxExportRfc822Controller) GetExport(request *http.Request) *http.Response {
	return ExportSourcesRoute{Parts: MailboxParts{
		Access: controller.accessHelper, Categories: controller.categoryMappingService,
		Emails: controller.emailsHelper, RawMessages: controller.rawMessagesHelper,
	}}.Export(request)
}
