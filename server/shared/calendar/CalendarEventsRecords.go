import "encoding/json"

// CalendarEventsRecords converts calendar event documents. FYLO cannot index
// an array of objects, so an event's attendees are stored as JSON text in
// attendeesJson and read back as a list.
type CalendarEventsRecords struct{}

// Hydrated answers one stored event with its attendees as a list.
func (CalendarEventsRecords) Hydrated(raw map[string]any) map[string]any {
	event := StoredRecords{}.Without(raw)
	if text, isText := event["attendeesJson"].(string); isText {
		var attendees any
		if json.Unmarshal([]byte(text), &attendees) != nil {
			attendees = []any{}
		}
		event["attendees"] = attendees
		delete(event, "attendeesJson")
	} else if _, isList := (RecordValues{}).List(event["attendees"]); !isList {
		event["attendees"] = []any{}
	}
	return event
}

// Stored answers the document of an event: without the repository's docId,
// and with its attendees as JSON text.
func (CalendarEventsRecords) Stored(event map[string]any) map[string]any {
	stored := StoredRecords{}.Without(event, "docId", "attendees")
	attendees, isList := RecordValues{}.List(event["attendees"])
	if !isList {
		attendees = []any{}
	}
	stored["attendeesJson"] = StoredRecords{}.JsonText(attendees)
	return stored
}
