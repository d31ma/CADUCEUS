import (
	"regexp"
	"strings"
)

// InviteCalendar reads the parts of an iCalendar attachment a message's
// invite card shows: the METHOD and, for each complete VEVENT (one with a
// UID, a SUMMARY and a DTSTART), its UID and attendees. Unlike the calendar
// importer it keeps no dates, recurrence or time zones.
type InviteCalendar struct {
	Method string
	Events []InviteEvent
}

// Parsed answers the calendar in text, or false when text is no VCALENDAR.
// It reads content lines the way the calendar importer's Ics does.
func (InviteCalendar) Parsed(text string) (InviteCalendar, bool) {
	if !regexp.MustCompile(`(?im)^BEGIN:VCALENDAR`).MatchString(text) {
		return InviteCalendar{}, false
	}
	calendar := InviteCalendar{Events: []InviteEvent{}}
	var current *InviteEvent
	for _, line := range (Ics{}).unfolded(text) {
		name, parameters, value := Ics{}.line(line)
		switch {
		case name == "BEGIN" && strings.EqualFold(value, "VEVENT"):
			current = &InviteEvent{Attendees: []map[string]any{}}
		case name == "END" && strings.EqualFold(value, "VEVENT"):
			if current != nil && current.Uid != "" && current.summary && current.start {
				calendar.Events = append(calendar.Events, *current)
			}
			current = nil
		case current == nil:
			if name == "METHOD" {
				calendar.Method = strings.ToUpper(value)
			}
		default:
			current.apply(name, parameters, value)
		}
	}
	return calendar, true
}
