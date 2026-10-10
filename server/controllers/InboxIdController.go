import "net/http"

// @RequestRoute("/inbox/{id}")
// @SignedIn
type InboxIdController struct {
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
func (controller *InboxIdController) GetEmail(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Read(id, request)
}

// @PutRoute
func (controller *InboxIdController) PutEmail(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Change(id, request)
}

// @DeleteRoute
func (controller *InboxIdController) DeleteEmail(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Remove(id, request)
}

func (controller *InboxIdController) route() InboxEmailRoute {
	return InboxEmailRoute{Parts: controller.mailboxReadingService.Completed(MailboxParts{
		Access: controller.accessHelper, Categories: controller.categoryMappingService, Emails: controller.emailsHelper,
	})}
}
