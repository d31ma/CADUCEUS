// IcsCalendar is one parsed VCALENDAR: its METHOD (REQUEST, CANCEL, REPLY
// or "" when absent), its PRODID and its complete VEVENTs. Each event holds
// the fields the JavaScript parser set: uid, summary, start, end, allDay,
// timeZone, attendees, rrule, recurrenceId, sequence, status and so on.
type IcsCalendar struct {
	Method string
	Prodid string
	Events []map[string]any
}
