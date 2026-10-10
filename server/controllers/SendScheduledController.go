import "net/http"

// @RequestRoute("/send/scheduled")
// @SignedIn
type SendScheduledController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	sendService *SendService
}

// @GetRoute
func (controller *SendScheduledController) GetScheduled(request *http.Request) *http.Response {
	return SendScheduledRoute{Access: controller.accessHelper, Send: controller.sendService}.List(request)
}

// @DeleteRoute
func (controller *SendScheduledController) DeleteScheduled(request *http.Request) *http.Response {
	return SendScheduledRoute{Access: controller.accessHelper, Send: controller.sendService}.CancelNamed(request)
}
