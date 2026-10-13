import "net/http"

// CalendarCalendarsRoute answers /calendar/calendars: the caller's provider
// calendars.
type CalendarCalendarsRoute struct {
	Access    *AccessHelper
	Providers *ProviderCalendarService
}

// List answers {calendars}, empty when provider calendars cannot be reached.
func (route CalendarCalendarsRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	if !route.Providers.Available() {
		return Answer{}.Ok(map[string]any{"calendars": []any{}})
	}
	calendars, err := route.Providers.Calendars(caller.Token)
	if err != nil {
		return ExternalAccounts{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"calendars": calendars})
}
