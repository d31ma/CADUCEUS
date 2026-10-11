import "net/http"

// @RequestRoute("/send/scheduled/{id}")
// @SignedIn
type SendScheduledIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	sendService *SendService
}

// @GetRoute
func (controller *SendScheduledIdController) GetScheduledMessage(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Detail(id, request)
}

// @PutRoute
func (controller *SendScheduledIdController) PutScheduledMessage(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Edit(id, request)
}

// @DeleteRoute
func (controller *SendScheduledIdController) DeleteScheduledMessage(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Cancel(id, request)
}

func (controller *SendScheduledIdController) route() SendScheduledRoute {
	return SendScheduledRoute{Access: controller.accessHelper, Send: controller.sendService}
}
