import "net/http"

// @RequestRoute("/presence/heartbeat")
// @SignedIn
type PresenceHeartbeatController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	presenceService *PresenceService
}

// @PostRoute
func (controller *PresenceHeartbeatController) PostHeartbeat(request *http.Request) *http.Response {
	presence := PresenceRoute{Access: controller.accessHelper, Presence: controller.presenceService}
	return presence.Heartbeat(request)
}
