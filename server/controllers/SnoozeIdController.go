import "net/http"

// @RequestRoute("/snooze/{id}")
// @SignedIn
type SnoozeIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	snoozeService *SnoozeService
}

// @DeleteRoute
func (controller *SnoozeIdController) DeleteSnooze(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return SnoozeRoute{Access: controller.accessHelper, Snooze: controller.snoozeService}.Undo(id, request)
}
