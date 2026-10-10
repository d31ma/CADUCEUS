import (
	"net/http"
	"strings"
)

// CalendarTeamEventsRoute answers /calendar/team-events: the events of
// every same-domain user who shares their calendar, redacted to busy time
// or shown in full as each chose. The caller's own events are not included.
type CalendarTeamEventsRoute struct {
	Access *AccessHelper
	Events *CalendarEventsHelper
	Users  *UsersHelper
}

// List answers the shared events, sorted by start.
func (route CalendarTeamEventsRoute) List(request *http.Request) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	viewerDomains := map[string]bool{}
	for _, domain := range caller.Domains {
		viewerDomains[strings.ToLower(domain)] = true
	}
	if len(caller.Domains) == 0 {
		return Answer{}.Ok([]any{})
	}
	users, err := route.Users.List()
	if err != nil {
		return RouteAccess{}.Failure(err)
	}
	sharers := []map[string]any{}
	for _, user := range users {
		if route.shares(user, caller.Email, viewerDomains) {
			sharers = append(sharers, user)
		}
	}
	start, end := RouteQuery{}.Value(request, "start"), RouteQuery{}.Value(request, "end")
	if (start != "") != (end != "") {
		return Answer{}.BadRequest("calendar range requires both start and end")
	}
	shown := []map[string]any{}
	for _, owner := range sharers {
		events, err := route.Events.List(RecordValues{}.Text(owner["email"]), start, end)
		if err != nil {
			return RouteAccess{}.Failure(err)
		}
		if start != "" && end != "" {
			expanded, problem := CalendarRecurrence{}.Expand(events, start, end)
			if problem != "" {
				return Answer{}.BadRequest(problem)
			}
			events = expanded
		}
		for _, event := range events {
			shown = append(shown, route.shared(owner, event))
		}
	}
	CalendarEventsRoute{}.sortByStart(shown)
	return Answer{}.Ok(shown)
}

// shares reports whether a user other than the caller shares a calendar
// with one of the caller's domains.
func (CalendarTeamEventsRoute) shares(user map[string]any, viewer string, viewerDomains map[string]bool) bool {
	values := RecordValues{}
	if strings.EqualFold(values.Text(user["email"]), viewer) || values.Or(user["calendarVisibility"], "private") == "private" {
		return false
	}
	domains, _ := values.List(user["domains"])
	for _, domain := range domains {
		if viewerDomains[strings.ToLower(values.String(domain))] {
			return true
		}
	}
	return false
}

// shared answers one event of a teammate, with only its busy time unless
// they share their calendar in full.
func (CalendarTeamEventsRoute) shared(owner map[string]any, event map[string]any) map[string]any {
	values := RecordValues{}
	isException := values.Truthy(event["seriesId"])
	if flag, isFlag := event["isException"].(bool); isFlag {
		isException = flag
	}
	visibility := owner["calendarVisibility"]
	shown := map[string]any{
		"id":              values.String(owner["email"]) + "::" + values.String(event["id"]),
		"ownerEmail":      owner["email"],
		"ownerName":       values.Or(owner["name"], ""),
		"ownerVisibility": visibility,
		"allDay":          values.Truthy(event["allDay"]),
		"seriesId":        values.Or(event["seriesId"], ""),
		"recurrenceId":    values.Or(event["recurrenceId"], ""),
		"isRecurring":     values.Truthy(event["isRecurring"]) || values.Truthy(event["recurrence"]) || values.Truthy(event["seriesId"]),
		"isException":     isException,
		"readOnly":        true,
		"source":          "team",
	}
	for _, field := range []string{"start", "end"} {
		if value, present := event[field]; present {
			shown[field] = value
		}
	}
	if visibility == "full" {
		if title, present := event["title"]; present {
			shown["title"] = title
		}
		shown["location"] = values.Or(event["location"], "")
		shown["description"] = values.Or(event["description"], "")
		shown["attendees"] = values.Or(event["attendees"], []any{})
		shown["originalSource"] = values.Or(event["source"], "manual")
		shown["status"] = values.Or(event["status"], "confirmed")
		return shown
	}
	shown["title"], shown["location"], shown["description"] = "Busy", "", ""
	shown["attendees"], shown["status"] = []any{}, "confirmed"
	return shown
}
