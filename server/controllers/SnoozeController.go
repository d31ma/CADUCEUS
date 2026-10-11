import "net/http"

// @RequestRoute("/snooze")
// @SignedIn
type SnoozeController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	snoozeService *SnoozeService
}

// @PostRoute
func (controller *SnoozeController) PostSnooze(request *http.Request) *http.Response {
	return SnoozeRoute{Access: controller.accessHelper, Snooze: controller.snoozeService}.Create(request)
}

// @GetRoute
func (controller *SnoozeController) GetSnoozes(request *http.Request) *http.Response {
	return SnoozeRoute{Access: controller.accessHelper, Snooze: controller.snoozeService}.List(request)
}
