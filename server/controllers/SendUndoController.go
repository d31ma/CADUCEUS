import "net/http"

// @RequestRoute("/send/undo")
// @SignedIn
type SendUndoController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	sendService *SendService
}

// @PostRoute
func (controller *SendUndoController) PostUndo(request *http.Request) *http.Response {
	return SendUndoRoute{Access: controller.accessHelper, Send: controller.sendService}.Undo(request)
}
