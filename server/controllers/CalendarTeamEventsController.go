import "net/http"

// @RequestRoute("/calendar/team-events")
// @SignedIn
type CalendarTeamEventsController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	calendarEventsHelper *CalendarEventsHelper
	// @Needs
	usersHelper *UsersHelper
}

// @GetRoute
func (controller *CalendarTeamEventsController) GetEvents(request *http.Request) *http.Response {
	return CalendarTeamEventsRoute{Access: controller.accessHelper, Events: controller.calendarEventsHelper, Users: controller.usersHelper}.List(request)
}
