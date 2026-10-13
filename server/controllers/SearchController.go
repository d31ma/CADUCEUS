import "net/http"

// @RequestRoute("/search")
// @SignedIn
type SearchController struct {
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
func (controller *SearchController) GetSearch(request *http.Request) *http.Response {
	return SearchRoute{Parts: controller.mailboxReadingService.Completed(MailboxParts{
		Access: controller.accessHelper, Categories: controller.categoryMappingService, Emails: controller.emailsHelper,
	})}.Find(request)
}
