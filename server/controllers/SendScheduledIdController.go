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
	return SendScheduledRoute{Access: controller.accessHelper, Send: controller.sendService}.Detail(id, request)
}

// @PutRoute
func (controller *SendScheduledIdController) PutScheduledMessage(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return SendScheduledRoute{Access: controller.accessHelper, Send: controller.sendService}.Edit(id, request)
}

// @DeleteRoute
func (controller *SendScheduledIdController) DeleteScheduledMessage(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return SendScheduledRoute{Access: controller.accessHelper, Send: controller.sendService}.Cancel(id, request)
}
