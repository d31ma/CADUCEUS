import "net/http"

// @RequestRoute("/calendar/invites/respond")
// @SignedIn
type CalendarInvitesRespondController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	calendarEventsHelper *CalendarEventsHelper
}

// @PostRoute
func (controller *CalendarInvitesRespondController) PostResponse(request *http.Request) *http.Response {
	return CalendarInvitesRoute{Access: controller.accessHelper, Events: controller.calendarEventsHelper}.Respond(request)
}
