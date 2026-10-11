import "net/http"

// @RequestRoute("/calendar/events")
// @SignedIn
type CalendarEventsController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	calendarEventsHelper *CalendarEventsHelper
	// @Needs
	providerCalendarService *ProviderCalendarService
}

// @PostRoute
func (controller *CalendarEventsController) PostEvent(request *http.Request) *http.Response {
	return controller.route().Create(request)
}

// @GetRoute
func (controller *CalendarEventsController) GetEvents(request *http.Request) *http.Response {
	return controller.route().List(request)
}

func (controller *CalendarEventsController) route() CalendarEventsRoute {
	return CalendarEventsRoute{Access: controller.accessHelper, Events: controller.calendarEventsHelper, Providers: controller.providerCalendarService}
}
