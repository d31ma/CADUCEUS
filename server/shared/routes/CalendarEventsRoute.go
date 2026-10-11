import (
	"math"
	"net/http"
	"sort"
)

// CalendarEventsRoute answers /calendar/events: the caller's events, with
// provider calendar events merged in, and new events.
type CalendarEventsRoute struct {
	Access    *AccessHelper
	Events    *CalendarEventsHelper
	Providers *ProviderCalendarService
}

// Create stores a new event on the caller's calendar, or on one of their
// provider calendars when the body names one.
func (route CalendarEventsRoute) Create(request *http.Request) *http.Response {
	body, refusal := RequestBody{}.Object(request)
	if refusal != nil {
		return refusal
	}
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	if body == nil {
		body = map[string]any{}
	}
	if allDay, present := body["allDay"]; present {
		if _, isFlag := allDay.(bool); !isFlag {
			return Answer{}.BadRequest("allDay must be a boolean")
		}
	}
	rules := CalendarEventRules{}
	title, start, end, problem := rules.Times(body)
	if problem != "" {
		return Answer{}.BadRequest(problem)
	}
	zone := "UTC"
	if text, isText := body["timeZone"].(string); isText {
		zone = text
	}
	if !(CalendarZone{}).Valid(zone) {
		return Answer{}.BadRequest("timeZone must be an IANA timezone")
	}
	rule, problem := CalendarRecurrence{}.Validate(body["recurrence"], start, zone)
	if problem != "" {
		return Answer{}.BadRequest(problem)
	}
	key := rules.IdempotencyKey(body["idempotencyKey"], "create")
	fields := StoredRecords{}.Without(body)
	fields["idempotencyKey"], fields["recurrence"], fields["timeZone"] = key, rule, zone
	event := rules.Built(fields, title, start, end, caller.Email)
	if calendarId := (Security{}).Trim(RecordValues{}.Text(body["calendarId"])); calendarId != "" {
		return route.createOnProvider(caller, calendarId, event, key)
	}
	_, replay, err := route.Events.FindByIdempotencyKey(caller.Email, key)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if replay != nil {
		return Answer{}.Ok(map[string]any{"saved": true, "id": replay["id"]})
	}
	if _, err := route.Events.Put(event); err != nil {
		return RouteAccess{}.Failure(err)
	}
	return Answer{}.Ok(map[string]any{"saved": true, "id": event["id"]})
}

func (route CalendarEventsRoute) createOnProvider(caller RouteCaller, calendarId string, event map[string]any, key string) *http.Response {
	if !route.Providers.Available() {
		return Answer{}.Unavailable("Provider calendar service unavailable")
	}
	mutation, err := route.Providers.Create(caller.Token, calendarId, event, key)
	if err != nil {
		return CalendarEventRules{}.ProviderFailure(err)
	}
	return Answer{}.Ok(map[string]any{"saved": true, "id": RecordValues{}.Or(mutation["resourceId"], mutation["id"])})
}

// List answers the caller's events sorted by start; with a start and an end
// it expands recurring series within that window.
func (route CalendarEventsRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	start, end := RouteQuery{}.Value(request, "start"), RouteQuery{}.Value(request, "end")
	if (start != "") != (end != "") {
		return Answer{}.BadRequest("calendar range requires both start and end")
	}
	events, err := route.Events.List(caller.Email, start, end)
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	if route.Providers.Available() {
		provided, err := route.Providers.List(caller.Token)
		if err != nil {
			return ExternalAccounts{}.Failure(err)
		}
		events = ProviderCalendarRules{}.Merged(events, provided)
	}
	if start != "" && end != "" {
		expanded, problem := CalendarRecurrence{}.Expand(events, start, end)
		if problem != "" {
			return Answer{}.BadRequest(problem)
		}
		events = expanded
	}
	route.sortByStart(events)
	now := EpochTime{}.Now()
	views := []map[string]any{}
	for _, event := range events {
		shown := StoredRecords{}.Without(event)
		shown["readOnly"] = event["readOnly"] == true || CalendarEventRules{}.ReadOnly(event, caller.Email) || (CalendarDate{}).IsPast(event, now)
		views = append(views, EventView{}.Of(shown))
	}
	return Answer{}.Ok(views)
}

// sortByStart orders events by their start instant, as the JavaScript sort
// did (events whose start does not parse keep their place).
func (CalendarEventsRoute) sortByStart(events []map[string]any) {
	sort.SliceStable(events, func(left, right int) bool {
		difference := EpochTime{}.Parse(RecordValues{}.String(events[left]["start"])) - EpochTime{}.Parse(RecordValues{}.String(events[right]["start"]))
		return !math.IsNaN(difference) && difference < 0
	})
}
