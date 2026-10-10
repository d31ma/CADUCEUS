import "net/http"

// @RequestRoute("/presence/team")
// @SignedIn
type PresenceTeamController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	presenceService *PresenceService
	// @Needs
	usersHelper *UsersHelper
}

// @GetRoute
func (controller *PresenceTeamController) GetTeam(request *http.Request) *http.Response {
	return PresenceRoute{Access: controller.accessHelper, Presence: controller.presenceService, Users: controller.usersHelper}.Team(request)
}
