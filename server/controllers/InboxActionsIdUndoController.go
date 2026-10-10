import "net/http"

// @RequestRoute("/inbox/actions/{id}/undo")
// @SignedIn
type InboxActionsIdUndoController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	mailActionService *MailActionService
}

// @PostRoute
func (controller *InboxActionsIdUndoController) PostUndo(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return MailActionsRoute{Access: controller.accessHelper, Actions: controller.mailActionService}.Undo(id, request)
}
