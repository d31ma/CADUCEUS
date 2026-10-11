// EventView is the public form of a calendar event that the collection and
// item routes answer. Document ids and owner fields never leave the server.
type EventView struct{}

// Of answers the public form of an event.
func (view EventView) Of(event map[string]any) map[string]any {
	values := RecordValues{}
	text := func(field string, fallback string) any { return values.Or(event[field], fallback) }
	list := func(field string) any {
		if items, isList := values.List(event[field]); isList {
			return items
		}
		return []any{}
	}
	isException := values.Truthy(event["seriesId"])
	if flag, isFlag := event["isException"].(bool); isFlag {
		isException = flag
	}
	var reminder any
	if minutes, isNumber := values.Number(event["reminderMinutes"]); isNumber {
		reminder = minutes
	}
	shown := map[string]any{
		"allDay":             values.Truthy(event["allDay"]),
		"location":           text("location", ""),
		"description":        text("description", ""),
		"attendees":          list("attendees"),
		"organizer":          text("organizer", ""),
		"source":             text("source", "manual"),
		"sourceEmailId":      text("sourceEmailId", ""),
		"icsUid":             text("icsUid", ""),
		"recurrence":         text("recurrence", ""),
		"recurrenceDates":    list("recurrenceDates"),
		"exceptionDates":     list("exceptionDates"),
		"timeZone":           text("timeZone", "UTC"),
		"seriesId":           text("seriesId", ""),
		"recurrenceId":       text("recurrenceId", ""),
		"isRecurring":        values.Truthy(event["isRecurring"]) || values.Truthy(event["recurrence"]) || values.Truthy(event["seriesId"]),
		"isException":        isException,
		"readOnly":           values.Truthy(event["readOnly"]),
		"reminderMinutes":    reminder,
		"status":             text("status", "confirmed"),
		"accountId":          text("accountId", ""),
		"accountLabel":       text("accountLabel", ""),
		"accountColor":       text("accountColor", ""),
		"calendarId":         text("calendarId", ""),
		"calendarName":       text("calendarName", ""),
		"calendarColor":      text("calendarColor", ""),
		"providerResourceId": text("providerResourceId", ""),
	}
	// Fields passed through as they are stay missing when the event has
	// none, as JSON.stringify dropped undefined.
	for _, field := range []string{"id", "title", "start", "end", "createdAt", "updatedAt"} {
		if value, present := event[field]; present {
			shown[field] = value
		}
	}
	return shown
}

// Listed answers the public form of every event.
func (view EventView) Listed(events []map[string]any) []map[string]any {
	views := []map[string]any{}
	for _, event := range events {
		views = append(views, view.Of(event))
	}
	return views
}
