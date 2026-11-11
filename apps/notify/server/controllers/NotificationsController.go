import "net/http"

// @RequestRoute("/api/v1/notify/notifications")
// @SignedIn
type NotificationsController struct {
	// @Needs
	notifications *NotificationsService
}

// @GetRoute
func (controller *NotificationsController) GetNotifications(request *http.Request) (NotificationPage, error) {
	query := request.URL.Query()
	return controller.notifications.List(request.Header.Get("Authorization"), query.Get("cursor"), query.Get("limit"))
}

// @PutRoute("/{notification_id}/read")
// @ResponseStatus(204)
func (controller *NotificationsController) PutRead(
	// @FromHeader("authorization")
	authorization string,
	// @FromPath("notification_id")
	notificationId string,
) error {
	return controller.notifications.MarkRead(authorization, notificationId)
}
