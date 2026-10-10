import "net/http"

// @RequestRoute("/inbox/actions/{id}/redo")
// @SignedIn
type InboxActionsIdRedoController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	mailActionService *MailActionService
}

// @PostRoute
func (controller *InboxActionsIdRedoController) PostRedo(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return MailActionsRoute{Access: controller.accessHelper, Actions: controller.mailActionService}.Redo(id, request)
}
