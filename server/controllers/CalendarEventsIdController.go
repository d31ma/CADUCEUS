import "net/http"

// @RequestRoute("/calendar/events/{id}")
// @SignedIn
type CalendarEventsIdController struct {
	// @Needs
	accessHelper *AccessHelper
	// @Needs
	calendarEventsHelper *CalendarEventsHelper
	// @Needs
	providerCalendarService *ProviderCalendarService
}

// @DeleteRoute
func (controller *CalendarEventsIdController) DeleteEvent(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Delete(id, request)
}

// @PutRoute
func (controller *CalendarEventsIdController) PutEvent(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Update(id, request)
}

// @GetRoute
func (controller *CalendarEventsIdController) GetEvent(
	// @FromPath
	id string,
	request *http.Request,
) *http.Response {
	return controller.route().Get(id, request)
}

func (controller *CalendarEventsIdController) route() CalendarEventRoute {
	return CalendarEventRoute{Access: controller.accessHelper, Events: controller.calendarEventsHelper, Providers: controller.providerCalendarService}
}
