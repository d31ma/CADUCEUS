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
func (InviteCalendar) Parsed(text string) (InviteCalendar, bool) {
	if !regexp.MustCompile(`(?im)^BEGIN:VCALENDAR`).MatchString(text) {
		return InviteCalendar{}, false
	}
	calendar := InviteCalendar{Events: []InviteEvent{}}
	var current *InviteEvent
	for _, line := range calendar.unfolded(text) {
		name, parameters, value := calendar.line(line)
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

func (InviteCalendar) unfolded(text string) []string {
	lines := []string{}
	for _, line := range strings.Split(regexp.MustCompile(`\r\n?|\n`).ReplaceAllString(text, "\n"), "\n") {
		if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && len(lines) > 0 {
			lines[len(lines)-1] += line[1:]
		} else {
			lines = append(lines, line)
		}
	}
	kept := []string{}
	for _, line := range lines {
		if line != "" {
			kept = append(kept, line)
		}
	}
	return kept
}

// line splits NAME[;PARAM=VALUE]*:VALUE, with quoted parameter values.
func (InviteCalendar) line(line string) (string, map[string]string, string) {
	index := 0
	for index < len(line) && line[index] != ';' && line[index] != ':' {
		index++
	}
	name := strings.ToUpper(line[:index])
	parameters := map[string]string{}
	for index < len(line) && line[index] == ';' {
		index++
		start := index
		for index < len(line) && line[index] != '=' {
			index++
		}
		parameter := strings.ToUpper(line[start:min(index, len(line))])
		index++
		var value string
		if index < len(line) && line[index] == '"' {
			index++
			start = index
			for index < len(line) && line[index] != '"' {
				index++
			}
			value = line[start:min(index, len(line))]
			index++
		} else {
			start = min(index, len(line))
			for index < len(line) && line[index] != ';' && line[index] != ':' {
				index++
			}
			value = line[start:min(index, len(line))]
		}
		parameters[parameter] = value
	}
	if index < len(line) && line[index] == ':' {
		index++
	}
	return name, parameters, line[min(index, len(line)):]
}
