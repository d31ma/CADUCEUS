import "net/http"

// @RequestRoute("/threads")
// @SignedIn
type ThreadsController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	categoryMappingService *CategoryMappingService
	// @Needs
	emailsHelper *EmailsHelper
	// @Needs
	mailboxReadingService *MailboxReadingService
}

// @GetRoute
func (controller *ThreadsController) GetThreads(request *http.Request) *http.Response {
	return ThreadsRoute{Parts: controller.mailboxReadingService.Completed(MailboxParts{
		Access: controller.accessHelper, Categories: controller.categoryMappingService, Emails: controller.emailsHelper,
	})}.List(request)
}
