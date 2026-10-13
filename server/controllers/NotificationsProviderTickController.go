import "net/http"

// @RequestRoute("/notifications/provider/tick")
// @Public
type NotificationsProviderTickController struct {
	// @Needs
	pushService *PushService
}

// PostTick processes one leased provider new-mail notification batch.
// @PostRoute
func (controller *NotificationsProviderTickController) PostTick(request *http.Request) *http.Response {
	return ProviderTickRoute{Push: controller.pushService}.Tick(request)
}
