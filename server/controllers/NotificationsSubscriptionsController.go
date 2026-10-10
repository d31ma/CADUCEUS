import "net/http"

// @RequestRoute("/notifications/subscriptions")
// @SignedIn
type NotificationsSubscriptionsController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	pushSubscriptionsHelper *PushSubscriptionsHelper
}

// @PostRoute
func (controller *NotificationsSubscriptionsController) PostSubscription(request *http.Request) *http.Response {
	return NotificationsRoute{Access: controller.accessHelper, Subscriptions: controller.pushSubscriptionsHelper}.Subscribe(request)
}

// @DeleteRoute
func (controller *NotificationsSubscriptionsController) DeleteSubscription(request *http.Request) *http.Response {
	return controller.notifications().Unsubscribe(request)
}

func (controller *NotificationsSubscriptionsController) notifications() NotificationsRoute {
	return NotificationsRoute{Access: controller.accessHelper, Subscriptions: controller.pushSubscriptionsHelper}
}

// @GetRoute
func (controller *NotificationsSubscriptionsController) GetSubscriptions(request *http.Request) *http.Response {
	notifications := controller.notifications()
	return notifications.List(request)
}
