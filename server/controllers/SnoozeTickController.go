import "net/http"

// @RequestRoute("/snooze/tick")
// @Public
type SnoozeTickController struct {
	// @Needs
	snoozeService *SnoozeService
}

// @PostRoute
func (controller *SnoozeTickController) PostTick(request *http.Request) *http.Response {
	return SnoozeRoute{Snooze: controller.snoozeService}.Tick(request)
}
