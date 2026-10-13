// Package calendarapi translates the bounded event subset shared by native
// provider JSON APIs and CADUCEUS's RFC 5545 persistence boundary.
package calendarapi

import (
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidCalendar = errors.New("provider calendar event is invalid")

type DateTime struct {
	Date     string
	Instant  time.Time
	TimeZone string
	AllDay   bool
}

type Attendee struct {
	Email  string
	Name   string
	Status string
}

type Event struct {
	UID             string
	Summary         string
	Description     string
	Location        string
	Status          string
	Organizer       string
	Start           DateTime
	End             DateTime
	RecurrenceID    *DateTime
	Recurrence      []string
	Attendees       []Attendee
	ReminderMinutes *int
}

func Render(event Event, now time.Time) (string, error) {
	if err := Validate(event); err != nil {
		return "", err
	}
	lines := []string{
		"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//DELMA//CADUCEUS//EN", "CALSCALE:GREGORIAN", "BEGIN:VEVENT",
		"UID:" + escapeText(event.UID), "DTSTAMP:" + now.UTC().Format("20060102T150405Z"),
		dateLine("DTSTART", event.Start), dateLine("DTEND", event.End),
	}
	if event.RecurrenceID != nil {
		lines = append(lines, dateLine("RECURRENCE-ID", *event.RecurrenceID))
	}
	if event.Summary != "" {
		lines = append(lines, "SUMMARY:"+escapeText(event.Summary))
	}
	if event.Description != "" {
		lines = append(lines, "DESCRIPTION:"+escapeText(event.Description))
	}
	if event.Location != "" {
		lines = append(lines, "LOCATION:"+escapeText(event.Location))
	}
	if event.Organizer != "" {
		lines = append(lines, "ORGANIZER:mailto:"+event.Organizer)
	}
	for _, attendee := range event.Attendees {
		params := ""
		if attendee.Name != "" {
			params += ";CN=" + quoteParam(attendee.Name)
		}
		if status := partstat(attendee.Status); status != "" {
			params += ";PARTSTAT=" + status
		}
		lines = append(lines, "ATTENDEE"+params+":mailto:"+attendee.Email)
	}
	for _, recurrence := range event.Recurrence {
		recurrence = strings.TrimSpace(recurrence)
		if recurrence != "" {
			lines = append(lines, recurrence)
		}
	}
	if event.Status != "" {
		lines = append(lines, "STATUS:"+strings.ToUpper(event.Status))
	}
	if event.ReminderMinutes != nil && *event.ReminderMinutes >= 0 {
		lines = append(lines, "BEGIN:VALARM", "ACTION:DISPLAY", fmt.Sprintf("TRIGGER:-PT%dM", *event.ReminderMinutes), "DESCRIPTION:Reminder", "END:VALARM")
	}
	lines = append(lines, "END:VEVENT", "END:VCALENDAR")
	return renderLines(lines), nil
}

func Parse(raw string) (Event, error) {
	if len(raw) == 0 || len(raw) > 2<<20 {
		return Event{}, ErrInvalidCalendar
	}
	lines := unfold(raw)
	inEvent, inAlarm := false, false
	var event Event
	for _, line := range lines {
		switch strings.ToUpper(strings.TrimSpace(line)) {
		case "BEGIN:VEVENT":
			if inEvent {
				return Event{}, ErrInvalidCalendar
			}
			inEvent = true
			continue
		case "END:VEVENT":
			if inEvent {
				if err := Validate(event); err != nil {
					return Event{}, err
				}
				return event, nil
			}
		case "BEGIN:VALARM":
			if inEvent {
				inAlarm = true
			}
			continue
		case "END:VALARM":
			inAlarm = false
			continue
		}
		if !inEvent {
			continue
		}
		property, params, value, ok := property(line)
		if !ok {
			continue
		}
		if inAlarm {
			if property == "TRIGGER" {
				if minutes, ok := parseTrigger(value); ok {
					event.ReminderMinutes = &minutes
				}
			}
			continue
		}
		switch property {
		case "UID":
			event.UID = unescapeText(value)
		case "SUMMARY":
			event.Summary = unescapeText(value)
		case "DESCRIPTION":
			event.Description = unescapeText(value)
		case "LOCATION":
			event.Location = unescapeText(value)
		case "STATUS":
			event.Status = strings.ToLower(strings.TrimSpace(value))
		case "ORGANIZER":
			event.Organizer = mailAddress(value)
		case "ATTENDEE":
			address := mailAddress(value)
			if address != "" {
				event.Attendees = append(event.Attendees, Attendee{Email: address, Name: unquoteParam(params["CN"]), Status: strings.ToLower(params["PARTSTAT"])})
			}
		case "DTSTART":
			parsed, err := parseDate(value, params)
			if err != nil {
				return Event{}, err
			}
			event.Start = parsed
		case "DTEND":
			parsed, err := parseDate(value, params)
			if err != nil {
				return Event{}, err
			}
			event.End = parsed
		case "RECURRENCE-ID":
			parsed, err := parseDate(value, params)
			if err != nil {
				return Event{}, err
			}
			event.RecurrenceID = &parsed
		case "RRULE", "RDATE", "EXDATE":
			event.Recurrence = append(event.Recurrence, strings.TrimSpace(line))
		}
	}
	return Event{}, ErrInvalidCalendar
}

func Validate(event Event) error {
	if invalidText(event.UID, 1024) || len(event.Summary) > 8192 || len(event.Description) > 256<<10 || len(event.Location) > 8192 ||
		strings.ContainsAny(event.Summary, "\x00") || strings.ContainsAny(event.Description, "\x00") || strings.ContainsAny(event.Location, "\x00") ||
		!validDate(event.Start) || !validDate(event.End) || event.Start.AllDay != event.End.AllDay || len(event.Attendees) > 500 || len(event.Recurrence) > 128 {
		return ErrInvalidCalendar
	}
	if event.Start.AllDay {
		if event.End.Date <= event.Start.Date {
			return ErrInvalidCalendar
		}
	} else if !event.End.Instant.After(event.Start.Instant) {
		return ErrInvalidCalendar
	}
	if event.RecurrenceID != nil && (!validDate(*event.RecurrenceID) || event.RecurrenceID.AllDay != event.Start.AllDay) {
		return ErrInvalidCalendar
	}
	if event.Status != "" && event.Status != "confirmed" && event.Status != "tentative" && event.Status != "cancelled" {
		return ErrInvalidCalendar
	}
	if event.Organizer != "" && !validEmail(event.Organizer) {
		return ErrInvalidCalendar
	}
	for _, attendee := range event.Attendees {
		if !validEmail(attendee.Email) || len(attendee.Name) > 1024 || strings.ContainsAny(attendee.Name, "\x00\r\n") {
			return ErrInvalidCalendar
		}
	}
	for _, recurrence := range event.Recurrence {
		if !validRecurrence(recurrence) {
			return ErrInvalidCalendar
		}
	}
	if event.ReminderMinutes != nil && (*event.ReminderMinutes < 0 || *event.ReminderMinutes > 40320) {
		return ErrInvalidCalendar
	}
	return nil
}

func NewDate(value string) (DateTime, error) {
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return DateTime{}, ErrInvalidCalendar
	}
	return DateTime{Date: value, AllDay: true}, nil
}

func NewInstant(value, zone string) (DateTime, error) {
	instant, err := time.Parse(time.RFC3339, value)
	if err != nil {
		// Microsoft Graph can omit an offset when the response timezone is UTC.
		instant, err = time.Parse("2006-01-02T15:04:05.9999999", strings.TrimSuffix(value, "Z"))
		if err == nil {
			instant = time.Date(instant.Year(), instant.Month(), instant.Day(), instant.Hour(), instant.Minute(), instant.Second(), instant.Nanosecond(), time.UTC)
		}
	}
	if err != nil {
		return DateTime{}, ErrInvalidCalendar
	}
	return DateTime{Instant: instant, TimeZone: normalizedZone(zone)}, nil
}

func DateString(value DateTime) string {
	if value.AllDay {
		return value.Date
	}
	return value.Instant.UTC().Format(time.RFC3339)
}

func dateLine(name string, value DateTime) string {
	if value.AllDay {
		return name + ";VALUE=DATE:" + strings.ReplaceAll(value.Date, "-", "")
	}
	zone := normalizedZone(value.TimeZone)
	if zone != "UTC" {
		if location, err := time.LoadLocation(zone); err == nil {
			return name + ";TZID=" + zone + ":" + value.Instant.In(location).Format("20060102T150405")
		}
	}
	return name + ":" + value.Instant.UTC().Format("20060102T150405Z")
}

func parseDate(value string, params map[string]string) (DateTime, error) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(params["VALUE"], "DATE") || len(value) == 8 && !strings.Contains(value, "T") {
		parsed, err := time.Parse("20060102", value)
		if err != nil {
			return DateTime{}, ErrInvalidCalendar
		}
		return DateTime{Date: parsed.Format("2006-01-02"), AllDay: true}, nil
	}
	zone := normalizedZone(params["TZID"])
	var parsed time.Time
	var err error
	if strings.HasSuffix(value, "Z") {
		parsed, err = time.Parse("20060102T150405Z", value)
		zone = "UTC"
	} else {
		location, loadErr := time.LoadLocation(zone)
		if loadErr != nil {
			location = time.UTC
			zone = "UTC"
		}
		parsed, err = time.ParseInLocation("20060102T150405", value, location)
	}
	if err != nil {
		return DateTime{}, ErrInvalidCalendar
	}
	return DateTime{Instant: parsed, TimeZone: zone}, nil
}

func property(line string) (string, map[string]string, string, bool) {
	colon := unquotedIndex(line, ':')
	if colon <= 0 {
		return "", nil, "", false
	}
	parts := splitUnquoted(line[:colon], ';')
	name := strings.ToUpper(strings.TrimSpace(parts[0]))
	params := map[string]string{}
	for _, raw := range parts[1:] {
		separator := unquotedIndex(raw, '=')
		if separator > 0 {
			params[strings.ToUpper(strings.TrimSpace(raw[:separator]))] = strings.TrimSpace(raw[separator+1:])
		}
	}
	return name, params, line[colon+1:], true
}

func unquotedIndex(value string, separator byte) int {
	quoted, escaped := false, false
	for index := 0; index < len(value); index++ {
		current := value[index]
		if escaped {
			escaped = false
			continue
		}
		if current == '\\' {
			escaped = true
			continue
		}
		if current == '"' {
			quoted = !quoted
			continue
		}
		if current == separator && !quoted {
			return index
		}
	}
	return -1
}

func splitUnquoted(value string, separator byte) []string {
	parts := []string{}
	for {
		index := unquotedIndex(value, separator)
		if index < 0 {
			return append(parts, value)
		}
		parts = append(parts, value[:index])
		value = value[index+1:]
	}
}

func unfold(raw string) []string {
	raw = strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	physical := strings.Split(raw, "\n")
	logical := make([]string, 0, len(physical))
	for _, line := range physical {
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') && len(logical) > 0 {
			logical[len(logical)-1] += line[1:]
		} else {
			logical = append(logical, line)
		}
	}
	return logical
}

func escapeText(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\r\n", "\\n")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\r", "\\n")
	value = strings.ReplaceAll(value, ";", "\\;")
	return strings.ReplaceAll(value, ",", "\\,")
}

func unescapeText(value string) string {
	var out strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' || index+1 >= len(value) {
			out.WriteByte(value[index])
			continue
		}
		index++
		switch value[index] {
		case 'n', 'N':
			out.WriteByte('\n')
		default:
			out.WriteByte(value[index])
		}
	}
	return out.String()
}

func quoteParam(value string) string {
	value = strings.ReplaceAll(strings.ReplaceAll(value, "\\", "\\\\"), "\"", "\\\"")
	return "\"" + value + "\""
}

func unquoteParam(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value = value[1 : len(value)-1]
	}
	return strings.ReplaceAll(strings.ReplaceAll(value, "\\\"", "\""), "\\\\", "\\")
}

func mailAddress(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "mailto:") {
		value = value[7:]
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Address)
}

func validEmail(value string) bool {
	parsed, err := mail.ParseAddress(value)
	return err == nil && strings.EqualFold(parsed.Address, value) && len(value) <= 320 && !strings.ContainsAny(value, "\x00\r\n")
}

func validDate(value DateTime) bool {
	if value.AllDay {
		_, err := time.Parse("2006-01-02", value.Date)
		return err == nil
	}
	return !value.Instant.IsZero()
}

func invalidText(value string, maximum int) bool {
	return strings.TrimSpace(value) == "" || len(value) > maximum || strings.ContainsAny(value, "\x00\r\n")
}

func validRecurrence(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 4096 || strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	property, _, raw, ok := property(value)
	return ok && strings.TrimSpace(raw) != "" && (property == "RRULE" || property == "RDATE" || property == "EXDATE")
}

func renderLines(lines []string) string {
	var output strings.Builder
	for _, line := range lines {
		for _, folded := range foldLine(line) {
			output.WriteString(folded)
			output.WriteString("\r\n")
		}
	}
	return output.String()
}

// foldLine applies RFC 5545's 75-octet content-line limit without splitting
// a UTF-8 code point. Continuation whitespace counts toward the next limit.
func foldLine(line string) []string {
	if len(line) <= 75 {
		return []string{line}
	}
	result := []string{}
	var segment strings.Builder
	bytes := 0
	for _, character := range line {
		encoded := string(character)
		if bytes+len(encoded) > 75 {
			result = append(result, segment.String())
			segment.Reset()
			segment.WriteByte(' ')
			bytes = 1
		}
		segment.WriteString(encoded)
		bytes += len(encoded)
	}
	return append(result, segment.String())
}

func normalizedZone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "Etc/UTC") || strings.EqualFold(value, "GMT") {
		return "UTC"
	}
	if _, err := time.LoadLocation(value); err != nil {
		return "UTC"
	}
	return value
}

func partstat(value string) string {
	switch strings.ToLower(value) {
	case "accepted":
		return "ACCEPTED"
	case "declined":
		return "DECLINED"
	case "tentative":
		return "TENTATIVE"
	case "delegated":
		return "DELEGATED"
	case "completed":
		return "COMPLETED"
	default:
		return "NEEDS-ACTION"
	}
}

func parseTrigger(value string) (int, bool) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if !strings.HasPrefix(value, "-PT") || !strings.HasSuffix(value, "M") {
		return 0, false
	}
	minutes, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(value, "-PT"), "M"))
	return minutes, err == nil && minutes >= 0 && minutes <= 40320
}

// RecurrenceParts parses one RRULE into normalized key/value components.
func RecurrenceParts(recurrence []string) map[string]string {
	parts := map[string]string{}
	for _, line := range recurrence {
		if !strings.HasPrefix(strings.ToUpper(line), "RRULE:") {
			continue
		}
		for _, token := range strings.Split(line[len("RRULE:"):], ";") {
			pair := strings.SplitN(token, "=", 2)
			if len(pair) == 2 {
				parts[strings.ToUpper(strings.TrimSpace(pair[0]))] = strings.ToUpper(strings.TrimSpace(pair[1]))
			}
		}
		break
	}
	return parts
}

func SortedRecurrence(recurrence []string) []string {
	copy := append([]string(nil), recurrence...)
	sort.Strings(copy)
	return copy
}
