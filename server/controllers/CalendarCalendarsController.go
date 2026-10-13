import "net/http"

// @RequestRoute("/calendar/calendars")
// @SignedIn
type CalendarCalendarsController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	providerCalendarService *ProviderCalendarService
}

// @GetRoute
func (controller *CalendarCalendarsController) GetCalendars(request *http.Request) *http.Response {
	return CalendarCalendarsRoute{Access: controller.accessHelper, Providers: controller.providerCalendarService}.List(request)
}
