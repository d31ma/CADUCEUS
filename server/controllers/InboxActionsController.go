import "net/http"

// @RequestRoute("/inbox/actions")
// @SignedIn
type InboxActionsController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	mailActionService *MailActionService
}

// @PostRoute
func (controller *InboxActionsController) PostAction(request *http.Request) *http.Response {
	return MailActionsRoute{Access: controller.accessHelper, Actions: controller.mailActionService}.Apply(request)
}
