import "net/http"

// @RequestRoute("/notifications/vapid-public-key")
// @SignedIn
type NotificationsVapidPublicKeyController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	pushService *PushService
}

// @GetRoute
func (controller *NotificationsVapidPublicKeyController) GetPublicKey(request *http.Request) *http.Response {
	return NotificationsRoute{Access: controller.accessHelper, Push: controller.pushService}.VapidPublicKey(request)
}
