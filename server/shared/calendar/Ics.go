import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Ics reads and writes the small iCalendar (RFC 5545) subset that meeting
// invitations and CalDAV resources use: VEVENTs with their recurrence
// identity, time zone, EXDATE and RDATE fields, inside a VCALENDAR whose
// METHOD says whether it is a REQUEST, a CANCEL or a REPLY.
type Ics struct{}

// LooksLike reports whether text holds a VCALENDAR (a line starting with
// BEGIN:VCALENDAR).
func (Ics) LooksLike(text string) bool {
	return regexp.MustCompile(`(?i)(?:^|[\n\r\x{2028}\x{2029}])BEGIN:VCALENDAR`).MatchString(text)
}

// Parse answers the calendar of a VCALENDAR payload; ok is false when the
// text is not one. An event whose start cannot be read fails the parse.
func (ics Ics) Parse(text string) (IcsCalendar, bool, error) {
	calendar := IcsCalendar{Events: []map[string]any{}}
	if !ics.LooksLike(text) {
		return calendar, false, nil
	}
	var current map[string]any
	inEvent := false
	for _, raw := range ics.unfolded(text) {
		name, parameters, value := ics.line(raw)
		if name == "BEGIN" && strings.ToUpper(value) == "VEVENT" {
			inEvent = true
			current = map[string]any{"attendees": []any{}, "allDay": false}
			continue
		}
		if name == "END" && strings.ToUpper(value) == "VEVENT" {
			if current != nil && ics.truthy(current["start"]) && !ics.truthy(current["end"]) {
				end, err := ics.implicitEnd(current["start"].(string), ics.truthy(current["allDay"]), RecordValues{}.Text(current["duration"]))
				if err != nil {
					return calendar, true, err
				}
				current["end"] = end
			}
			if current != nil && ics.truthy(current["uid"]) && ics.truthy(current["summary"]) && ics.truthy(current["start"]) && ics.truthy(current["end"]) {
				calendar.Events = append(calendar.Events, current)
			}
			inEvent = false
			current = nil
			continue
		}
		if !inEvent {
			if name == "METHOD" {
				calendar.Method = strings.ToUpper(value)
			}
			if name == "PRODID" {
				calendar.Prodid = value
			}
			continue
		}
		if current != nil {
			ics.apply(current, name, parameters, value)
		}
	}
	return calendar, true, nil
}

// Reply answers a VCALENDAR REPLY that tells an organizer one attendee's
// answer (accepted, declined or tentative).
func (ics Ics) Reply(reply map[string]any) string {
	values := RecordValues{}
	partstat := map[string]string{"accepted": "ACCEPTED", "declined": "DECLINED"}[values.Text(reply["status"])]
	if partstat == "" {
		partstat = "TENTATIVE"
	}
	name := ""
	if attendeeName := values.Text(reply["attendeeName"]); attendeeName != "" {
		name = `;CN="` + strings.ReplaceAll(attendeeName, `"`, `\"`) + `"`
	}
	start, end := values.Text(reply["start"]), values.Text(reply["end"])
	startLine, endLine := "DTSTART:"+ics.instant(start), "DTEND:"+ics.instant(end)
	if values.Truthy(reply["allDay"]) {
		startLine, endLine = "DTSTART;VALUE=DATE:"+strings.ReplaceAll(start, "-", ""), "DTEND;VALUE=DATE:"+strings.ReplaceAll(end, "-", "")
	}
	return strings.Join([]string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//Caduceus//Mail Calendar//EN",
		"METHOD:REPLY",
		"BEGIN:VEVENT",
		"UID:" + values.Text(reply["uid"]),
		"DTSTAMP:" + ics.instant(EpochTime{}.NowText()),
		startLine,
		endLine,
		"SUMMARY:" + values.String(reply["summary"]),
		"ORGANIZER:mailto:" + values.Text(reply["organizer"]),
		"ATTENDEE" + name + ";PARTSTAT=" + partstat + ":mailto:" + values.Text(reply["attendee"]),
		"END:VEVENT",
		"END:VCALENDAR",
	}, "\r\n")
}

// Resource answers one writable event as a standards-based CalDAV resource.
func (ics Ics) Resource(event map[string]any, uid string) (string, error) {
	if uid == "" || strings.ContainsAny(uid, "\r\n\x00") {
		return "", errors.New("calendar UID is invalid")
	}
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//Caduceus//Mail Calendar//EN", "CALSCALE:GREGORIAN"}
	lines = append(lines, ics.eventLines(event, uid, RecordValues{}.Text(RecordValues{}.Or(event["recurrenceId"], "")), 0)...)
	return ics.folded(append(lines, "END:VCALENDAR", "")), nil
}

// Updated replaces a master or an exception in a CalDAV resource while
// keeping unrelated VEVENTs, VTIMEZONE, alarms and provider properties.
func (ics Ics) Updated(raw string, uid string, recurrenceId string, event map[string]any) (string, error) {
	if _, isCalendar, err := ics.Parse(raw); err != nil || !isCalendar {
		return "", ics.invalid(err)
	}
	lines := ics.unfolded(raw)
	blocks, err := ics.blocks(lines)
	if err != nil {
		return "", err
	}
	var found *IcsBlock
	for index := range blocks {
		block := blocks[index]
		if block.Event != nil && block.Event["uid"] == uid && (RecordValues{}).Text(RecordValues{}.Or(block.Event["recurrenceId"], "")) == recurrenceId {
			found = &blocks[index]
			break
		}
	}
	if found != nil {
		sequence := 1
		if number, isNumber := found.Event["sequence"].(int); isNumber {
			sequence = number + 1
		}
		replacement := ics.merged(lines[found.Start:found.End+1], event, uid, recurrenceId, sequence)
		lines = append(append(append([]string{}, lines[:found.Start]...), replacement...), lines[found.End+1:]...)
		return ics.folded(append(lines, "")), nil
	}
	hasMaster := false
	for _, block := range blocks {
		if block.Event != nil && block.Event["uid"] == uid && !ics.truthy(block.Event["recurrenceId"]) {
			hasMaster = true
		}
	}
	if recurrenceId == "" || !hasMaster {
		return "", errors.New("provider calendar event was not found")
	}
	end := -1
	for index, line := range lines {
		if line == "END:VCALENDAR" {
			end = index
		}
	}
	if end < 0 {
		return "", errors.New("provider calendar resource is invalid")
	}
	added := ics.eventLines(event, uid, recurrenceId, 1)
	lines = append(append(append([]string{}, lines[:end]...), added...), lines[end:]...)
	return ics.folded(append(lines, "")), nil
}

// Deleted removes a whole UID, or excludes one occurrence of its series.
// It answers deleteAll when nothing would be left of the resource.
func (ics Ics) Deleted(raw string, uid string, recurrenceId string) (string, bool, error) {
	calendar, isCalendar, err := ics.Parse(raw)
	if err != nil || !isCalendar {
		return "", false, ics.invalid(err)
	}
	if recurrenceId != "" {
		var master map[string]any
		for _, event := range calendar.Events {
			if event["uid"] == uid && !ics.truthy(event["recurrenceId"]) {
				master = event
				break
			}
		}
		if master == nil {
			return "", false, errors.New("provider calendar series was not found")
		}
		excluded := []any{}
		seen := map[string]bool{}
		dates, _ := RecordValues{}.List(master["exceptionDates"])
		for _, date := range append(append([]any{}, dates...), recurrenceId) {
			if key := (RecordValues{}).String(date); !seen[key] {
				seen[key] = true
				excluded = append(excluded, date)
			}
		}
		updated := StoredRecords{}.Without(master)
		updated["recurrence"] = RecordValues{}.Or(master["rrule"], "")
		updated["exceptionDates"] = excluded
		resource, err := ics.Updated(raw, uid, "", updated)
		return resource, false, err
	}
	lines := ics.unfolded(raw)
	blocks, err := ics.blocks(lines)
	if err != nil {
		return "", false, err
	}
	removed := map[int]bool{}
	for _, block := range blocks {
		if block.Event != nil && block.Event["uid"] == uid {
			for index := block.Start; index <= block.End; index++ {
				removed[index] = true
			}
		}
	}
	if len(removed) == 0 {
		return "", false, errors.New("provider calendar event was not found")
	}
	kept := []string{}
	for index, line := range lines {
		if !removed[index] {
			kept = append(kept, line)
		}
	}
	remaining, err := ics.blocks(kept)
	if err != nil {
		return "", false, err
	}
	if len(remaining) == 0 {
		return "", true, nil
	}
	return ics.folded(append(kept, "")), false, nil
}

// apply sets one VEVENT property on the event being read.
func (ics Ics) apply(event map[string]any, name string, parameters map[string]string, value string) {
	zone := func() string {
		if parameters["TZID"] != "" {
			return parameters["TZID"]
		}
		return RecordValues{}.Text(RecordValues{}.Or(event["timeZone"], "UTC"))
	}
	isDate := parameters["VALUE"] == "DATE"
	switch name {
	case "UID":
		event["uid"] = value
	case "SUMMARY":
		event["summary"] = ics.unescaped(value)
	case "DESCRIPTION":
		event["description"] = ics.unescaped(value)
	case "LOCATION":
		event["location"] = ics.unescaped(value)
	case "DTSTART":
		event["timeZone"] = "UTC"
		if parameters["TZID"] != "" && (CalendarZone{}).Valid(parameters["TZID"]) {
			event["timeZone"] = parameters["TZID"]
		}
		event["allDay"] = isDate
		if isDate {
			event["start"] = CalendarDate{}.FromCompact(value)
		} else {
			event["start"] = CalendarZone{}.ParseDateTime(value, event["timeZone"].(string))
		}
	case "DTEND":
		if isDate {
			event["end"] = CalendarDate{}.FromCompact(value)
			event["allDay"] = true
		} else {
			event["end"] = CalendarZone{}.ParseDateTime(value, zone())
		}
	case "DTSTAMP":
		event["dtstamp"] = CalendarZone{}.ParseDateTime(value, RecordValues{}.Text(RecordValues{}.Or(parameters["TZID"], "UTC")))
	case "ORGANIZER":
		event["organizer"] = ics.email(value)
	case "ATTENDEE":
		email := ics.email(value)
		if email == "" {
			return
		}
		status := ics.partstats()[parameters["PARTSTAT"]]
		if status == "" {
			status = "needsAction"
		}
		attendees, _ := RecordValues{}.List(event["attendees"])
		event["attendees"] = append(attendees, map[string]any{"email": email, "name": parameters["CN"], "status": status})
	case "RRULE":
		event["rrule"] = value
	case "RDATE", "EXDATE":
		field := map[string]string{"RDATE": "recurrenceDates", "EXDATE": "exceptionDates"}[name]
		dates, _ := RecordValues{}.List(event[field])
		event[field] = append(append([]any{}, dates...), ics.dateList(value, zone(), isDate)...)
	case "RECURRENCE-ID":
		if isDate {
			event["recurrenceId"] = CalendarDate{}.FromCompact(value)
		} else {
			event["recurrenceId"] = CalendarZone{}.ParseDateTime(value, zone())
		}
		if parameters["RANGE"] == "THISANDFUTURE" {
			event["recurrenceRange"] = "THISANDFUTURE"
		}
	case "SEQUENCE":
		if regexp.MustCompile(`^\d+$`).MatchString(value) {
			event["sequence"], _ = strconv.Atoi(value)
		}
	case "DURATION":
		event["duration"] = value
	case "STATUS":
		status := map[string]string{"CONFIRMED": "confirmed", "CANCELLED": "cancelled", "TENTATIVE": "tentative"}[strings.ToUpper(value)]
		if status == "" {
			delete(event, "status")
		} else {
			event["status"] = status
		}
	}
}

// unfolded answers the logical lines of text: a line starting with a space
// or a tab continues the line before it. Empty lines are dropped.
func (Ics) unfolded(text string) []string {
	normalized := regexp.MustCompile(`\r\n?|\n`).ReplaceAllString(text, "\n")
	lines := []string{}
	for _, line := range strings.Split(normalized, "\n") {
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

// line splits a content line into its upper-case name, its parameters and
// its value: NAME[;PARAM=VALUE]*:VALUE, where a quoted parameter value may
// hold semicolons and colons.
func (Ics) line(line string) (string, map[string]string, string) {
	at := func(index int) byte {
		if index < len(line) {
			return line[index]
		}
		return 0
	}
	index := 0
	for index < len(line) && line[index] != ';' && line[index] != ':' {
		index++
	}
	name := strings.ToUpper(line[:index])
	parameters := map[string]string{}
	for at(index) == ';' {
		index++
		start := index
		for index < len(line) && line[index] != '=' {
			index++
		}
		parameter := strings.ToUpper(line[start:index])
		index++
		value := ""
		if at(index) == '"' {
			index++
			start = index
			for index < len(line) && line[index] != '"' {
				index++
			}
			value = line[start:index]
			index++
		} else {
			start = index
			for index < len(line) && line[index] != ';' && line[index] != ':' {
				index++
			}
			value = line[min(start, len(line)):min(index, len(line))]
		}
		parameters[parameter] = value
	}
	if at(index) == ':' {
		index++
	}
	return name, parameters, line[min(index, len(line)):]
}

func (Ics) dateList(value string, zone string, asDate bool) []any {
	dates := []any{}
	for _, item := range strings.Split(value, ",") {
		if asDate {
			if date := (CalendarDate{}).FromCompact(item); (CalendarDate{}).IsDate(date) {
				dates = append(dates, date)
			}
		} else if instant := (CalendarZone{}).ParseDateTime(item, zone); (CalendarDate{}).IsDateTime(instant) {
			dates = append(dates, instant)
		}
	}
	return dates
}

func (ics Ics) implicitEnd(start string, allDay bool, duration string) (string, error) {
	day := float64(24 * 60 * 60 * 1000)
	length := ics.duration(duration)
	if allDay && (CalendarDate{}).IsDate(start) {
		if length == 0 {
			length = day
		}
		return CalendarDate{}.AddDays(start, int(math.Max(1, math.Round(length/day)))), nil
	}
	base := EpochTime{}.Parse(start)
	if length == 0 && allDay {
		length = day
	}
	if math.IsNaN(base) {
		return "", errors.New("Invalid time value")
	}
	return EpochTime{}.Text(int64(base + length)), nil
}

// duration answers an RFC 5545 DURATION in milliseconds (0 when it does not
// read).
func (Ics) duration(value string) float64 {
	parts := regexp.MustCompile(`(?i)^P(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`).FindStringSubmatch(value)
	if value == "" || parts == nil {
		return 0
	}
	number := func(text string) float64 {
		parsed, _ := strconv.ParseFloat(text, 64)
		return parsed
	}
	return (((number(parts[1])*7+number(parts[2]))*24+number(parts[3]))*60*60 + number(parts[4])*60 + number(parts[5])) * 1000
}

// instant answers an instant as an RFC 5545 UTC DATE-TIME.
func (Ics) instant(iso string) string {
	parsed := EpochTime{}.Parse(iso)
	if math.IsNaN(parsed) {
		return "NaNNaNNaNTNaNNaNNaNZ"
	}
	moment := time.UnixMilli(int64(parsed)).UTC()
	return fmt.Sprintf("%d%02d%02dT%02d%02d%02dZ", moment.Year(), int(moment.Month()), moment.Day(), moment.Hour(), moment.Minute(), moment.Second())
}

// date answers a calendar day as an RFC 5545 DATE.
func (Ics) date(value any) string {
	text := strings.ReplaceAll(RecordValues{}.String(RecordValues{}.Or(value, "")), "-", "")
	if len(text) > 8 {
		return text[:8]
	}
	return text
}

func (Ics) escaped(value any) string {
	text := RecordValues{}.String(RecordValues{}.Or(value, ""))
	text = strings.ReplaceAll(text, `\`, `\\`)
	text = regexp.MustCompile(`\r?\n`).ReplaceAllString(text, `\n`)
	text = strings.ReplaceAll(text, ";", `\;`)
	return strings.ReplaceAll(text, ",", `\,`)
}

func (Ics) unescaped(value string) string {
	value = regexp.MustCompile(`(?i)\\n`).ReplaceAllString(value, "\n")
	value = strings.ReplaceAll(value, `\,`, ",")
	value = strings.ReplaceAll(value, `\;`, ";")
	return strings.ReplaceAll(value, `\\`, `\`)
}

func (Ics) parameter(value any) string {
	return regexp.MustCompile(`["\r\n]`).ReplaceAllString(RecordValues{}.String(RecordValues{}.Or(value, "")), "")
}

func (Ics) email(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToLower(regexp.MustCompile(`(?i)^mailto:`).ReplaceAllString(Security{}.Trim(value), ""))
}

// eventLines answers the content lines of one VEVENT.
func (ics Ics) eventLines(event map[string]any, uid string, recurrenceId string, sequence int) []string {
	values := RecordValues{}
	allDay := event["allDay"] == true
	when := func(property string, value any) string {
		if allDay {
			return property + ";VALUE=DATE:" + ics.date(value)
		}
		return property + ":" + ics.instant(values.String(value))
	}
	lines := []string{"BEGIN:VEVENT", "UID:" + uid, "DTSTAMP:" + ics.instant(EpochTime{}.NowText()), "SEQUENCE:" + strconv.Itoa(max(0, sequence))}
	if recurrenceId != "" {
		lines = append(lines, when("RECURRENCE-ID", recurrenceId))
	}
	lines = append(lines, when("DTSTART", event["start"]), when("DTEND", event["end"]))
	lines = append(lines, "SUMMARY:"+ics.escaped(values.Or(values.Or(event["title"], event["summary"]), "Untitled event")))
	if values.Truthy(event["location"]) {
		lines = append(lines, "LOCATION:"+ics.escaped(event["location"]))
	}
	if values.Truthy(event["description"]) {
		lines = append(lines, "DESCRIPTION:"+ics.escaped(event["description"]))
	}
	if values.Truthy(event["organizer"]) {
		lines = append(lines, "ORGANIZER:mailto:"+strings.ToLower(values.String(event["organizer"])))
	}
	attendees, _ := values.List(event["attendees"])
	for _, entry := range attendees {
		attendee, _ := values.Object(entry)
		if attendee == nil || !values.Truthy(attendee["email"]) {
			continue
		}
		partstat := map[string]string{"accepted": "ACCEPTED", "declined": "DECLINED", "tentative": "TENTATIVE", "needsAction": "NEEDS-ACTION", "delegated": "DELEGATED", "completed": "COMPLETED"}[values.Text(attendee["status"])]
		if partstat == "" {
			partstat = "NEEDS-ACTION"
		}
		parameters := "PARTSTAT=" + partstat
		if values.Truthy(attendee["name"]) {
			parameters = `CN="` + ics.parameter(attendee["name"]) + `";` + parameters
		}
		lines = append(lines, "ATTENDEE;"+parameters+":mailto:"+strings.ToLower(values.String(attendee["email"])))
	}
	if recurrence := values.Or(values.Or(event["recurrence"], event["rrule"]), ""); values.Truthy(recurrence) {
		lines = append(lines, "RRULE:"+values.String(recurrence))
	}
	list := func(property string, field string) {
		dates, _ := values.List(event[field])
		if len(dates) == 0 {
			return
		}
		formatted := []string{}
		for _, date := range dates {
			if allDay {
				formatted = append(formatted, ics.date(date))
			} else {
				formatted = append(formatted, ics.instant(values.String(date)))
			}
		}
		if allDay {
			property += ";VALUE=DATE"
		}
		lines = append(lines, property+":"+strings.Join(formatted, ","))
	}
	list("RDATE", "recurrenceDates")
	list("EXDATE", "exceptionDates")
	if values.Truthy(event["status"]) {
		lines = append(lines, "STATUS:"+strings.ToUpper(values.String(event["status"])))
	}
	return append(lines, "END:VEVENT")
}

// blocks answers the top-level VEVENT blocks of a resource's lines.
func (ics Ics) blocks(lines []string) ([]IcsBlock, error) {
	blocks := []IcsBlock{}
	start, depth := -1, 0
	for index, line := range lines {
		upper := strings.ToUpper(line)
		if upper == "BEGIN:VEVENT" {
			if depth == 0 {
				start = index
			}
			depth++
		} else if upper == "END:VEVENT" && depth > 0 {
			depth--
			if depth == 0 && start >= 0 {
				wrapped := strings.Join(append(append([]string{"BEGIN:VCALENDAR"}, lines[start:index+1]...), "END:VCALENDAR"), "\r\n")
				calendar, _, err := ics.Parse(wrapped)
				if err != nil {
					return nil, err
				}
				block := IcsBlock{Start: start, End: index}
				if len(calendar.Events) > 0 {
					block.Event = calendar.Events[0]
				}
				blocks = append(blocks, block)
				start = -1
			}
		}
	}
	return blocks, nil
}

// merged answers a replacement VEVENT block: the managed properties written
// from the event, every other property (and nested component) kept.
func (ics Ics) merged(block []string, event map[string]any, uid string, recurrenceId string, sequence int) []string {
	managed := map[string]bool{}
	for _, name := range strings.Split("UID DTSTAMP SEQUENCE RECURRENCE-ID DTSTART DTEND SUMMARY LOCATION DESCRIPTION ORGANIZER ATTENDEE RRULE RDATE EXDATE STATUS", " ") {
		managed[name] = true
	}
	preserved := []string{}
	nested := 0
	if len(block) >= 2 {
		for _, line := range block[1 : len(block)-1] {
			name, _, value := ics.line(line)
			if name == "BEGIN" {
				nested++
			}
			if nested > 0 || !managed[name] {
				preserved = append(preserved, line)
			}
			if name == "END" && strings.ToUpper(value) != "VEVENT" {
				nested = max(0, nested-1)
			}
		}
	}
	encoded := ics.eventLines(event, uid, recurrenceId, sequence)
	merged := append([]string{}, encoded[:len(encoded)-1]...)
	return append(append(merged, preserved...), encoded[len(encoded)-1])
}

// folded answers lines folded to 75 octets and joined with CRLF.
func (ics Ics) folded(lines []string) string {
	output := []string{}
	for _, line := range lines {
		output = append(output, ics.fold(line)...)
	}
	return strings.Join(output, "\r\n")
}

func (Ics) fold(line string) []string {
	if len(line) <= 75 {
		return []string{line}
	}
	output := []string{}
	remaining := line
	limit := 75
	for len(remaining) > limit {
		boundary := 0
		for boundary < len(remaining) {
			_, width := utf8.DecodeRuneInString(remaining[boundary:])
			if boundary+width > limit {
				break
			}
			boundary += width
		}
		prefix := ""
		if len(output) > 0 {
			prefix = " "
		}
		output = append(output, prefix+remaining[:boundary])
		remaining = remaining[boundary:]
		limit = 74
	}
	prefix := ""
	if len(output) > 0 {
		prefix = " "
	}
	return append(output, prefix+remaining)
}

func (Ics) partstats() map[string]string {
	return map[string]string{"ACCEPTED": "accepted", "DECLINED": "declined", "TENTATIVE": "tentative", "NEEDS-ACTION": "needsAction", "DELEGATED": "delegated", "COMPLETED": "completed"}
}

func (Ics) invalid(err error) error {
	if err != nil {
		return err
	}
	return errors.New("provider calendar resource is invalid")
}

func (Ics) truthy(value any) bool { return RecordValues{}.Truthy(value) }
