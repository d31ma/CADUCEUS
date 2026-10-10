import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// CalendarRecurrence expands stored RFC 5545 recurrence rules into bounded
// calendar instances. Each generated wall-clock time goes through the
// event's IANA time zone, so a 09:00 meeting stays at 09:00 across daylight
// saving changes. A stable seriesId and recurrenceId let one occurrence be
// replaced by an edited or cancelled exception document.
type CalendarRecurrence struct{}

// Validate answers the canonical RRULE of input ("" for none), or the
// problem with it. Sub-daily rules are not supported.
func (recurrence CalendarRecurrence) Validate(input any, start any, timeZone any) (string, string) {
	if input == nil || (Security{}).Trim(RecordValues{}.String(input)) == "" {
		return "", ""
	}
	value := Security{}.Trim(RecordValues{}.String(input))
	if len(value) >= len("RRULE:") && strings.EqualFold(value[:len("RRULE:")], "RRULE:") {
		value = value[len("RRULE:"):]
	}
	if len(value) > recurrence.longestRule() || strings.ContainsAny(value, "\r\n") {
		return "", "recurrence rule is too long or contains multiple lines"
	}
	if (RecordValues{}).Truthy(start) && !(CalendarDate{}).IsDate(start) && !(CalendarDate{}).IsDateTime(start) {
		return "", "event start is invalid"
	}
	if !(CalendarZone{}).Valid(RecordValues{}.Or(timeZone, "UTC")) {
		return "", "timeZone must be an IANA timezone"
	}
	parts := map[string]string{}
	for _, raw := range strings.Split(value, ";") {
		separator := strings.Index(raw, "=")
		if separator <= 0 || separator == len(raw)-1 {
			return "", "recurrence rule contains an invalid part"
		}
		key := strings.ToUpper(Security{}.Trim(raw[:separator]))
		part := strings.ToUpper(Security{}.Trim(raw[separator+1:]))
		if !recurrence.allowed(key) {
			return "", fmt.Sprintf("recurrence rule part %s is unsupported", key)
		}
		if _, seen := parts[key]; seen {
			return "", fmt.Sprintf("recurrence rule part %s is duplicated", key)
		}
		parts[key] = part
	}
	if problem := recurrence.problem(parts); problem != "" {
		return "", problem
	}
	ordered := []string{}
	for _, key := range recurrence.keys() {
		if part, present := parts[key]; present {
			ordered = append(ordered, key+"="+part)
		}
	}
	return strings.Join(ordered, ";"), ""
}

func (recurrence CalendarRecurrence) problem(parts map[string]string) string {
	weekday := `(?:MO|TU|WE|TH|FR|SA|SU)`
	has := func(key string) bool { _, present := parts[key]; return present }
	frequency := parts["FREQ"]
	switch {
	case !(frequency == "DAILY" || frequency == "WEEKLY" || frequency == "MONTHLY" || frequency == "YEARLY"):
		return "recurrence frequency must be DAILY, WEEKLY, MONTHLY, or YEARLY"
	case !recurrence.bounded(parts, "INTERVAL", 1, recurrence.largestInterval()):
		return fmt.Sprintf("recurrence interval must be from 1 through %d", recurrence.largestInterval())
	case !recurrence.bounded(parts, "COUNT", 1, recurrence.largestCount()):
		return fmt.Sprintf("recurrence count must be from 1 through %d", recurrence.largestCount())
	case has("COUNT") && has("UNTIL"):
		return "recurrence cannot contain both COUNT and UNTIL"
	case has("UNTIL") && !regexp.MustCompile(`^\d{8}(?:T\d{6}Z?)?$`).MatchString(parts["UNTIL"]):
		return "recurrence UNTIL is invalid"
	case has("WKST") && !regexp.MustCompile(`^`+weekday+`$`).MatchString(parts["WKST"]):
		return "recurrence WKST is invalid"
	case has("BYDAY") && !recurrence.listMatches(parts["BYDAY"], regexp.MustCompile(`^[+-]?(?:[1-9]|[1-4]\d|5[0-3])?`+weekday+`$`)):
		return "recurrence BYDAY is invalid"
	case has("BYMONTHDAY") && !recurrence.integers(parts["BYMONTHDAY"], -31, 31, false):
		return "recurrence BYMONTHDAY is invalid"
	case has("BYYEARDAY") && !recurrence.integers(parts["BYYEARDAY"], -366, 366, false):
		return "recurrence BYYEARDAY is invalid"
	case has("BYWEEKNO") && !recurrence.integers(parts["BYWEEKNO"], -53, 53, false):
		return "recurrence BYWEEKNO is invalid"
	case has("BYMONTH") && !recurrence.integers(parts["BYMONTH"], 1, 12, true):
		return "recurrence BYMONTH is invalid"
	case has("BYSETPOS") && !recurrence.integers(parts["BYSETPOS"], -366, 366, false):
		return "recurrence BYSETPOS is invalid"
	}
	return ""
}

// Expand answers the events visible in a window: ordinary events that
// overlap it, every occurrence of each recurring master, its exceptions in
// place of the occurrences they replace, and orphaned exceptions. It answers
// the problem instead when the window is invalid or too large.
func (recurrence CalendarRecurrence) Expand(events []map[string]any, start string, end string) ([]map[string]any, string) {
	windowStart := EpochTime{}.Parse(start)
	windowEnd := EpochTime{}.Parse(end)
	if math.IsNaN(windowStart) || math.IsNaN(windowEnd) || windowEnd < windowStart {
		return nil, "calendar range is invalid"
	}
	if windowEnd-windowStart > float64(recurrence.longestRangeDays())*recurrence.dayMilliseconds() {
		return nil, fmt.Sprintf("calendar range cannot exceed %d days", recurrence.longestRangeDays())
	}
	masters, exceptions := []map[string]any{}, []map[string]any{}
	for _, event := range events {
		if (RecordValues{}).Truthy(event["seriesId"]) {
			exceptions = append(exceptions, event)
		} else {
			masters = append(masters, event)
		}
	}
	bySeries := recurrence.groupExceptions(exceptions)
	known := map[string]bool{}
	for _, master := range masters {
		known[RecordValues{}.String(master["id"])] = true
	}
	output := []map[string]any{}
	for _, master := range masters {
		if master["status"] == "cancelled" {
			continue
		}
		rule, problem := recurrence.Validate(master["recurrence"], master["start"], master["timeZone"])
		if problem != "" || rule == "" {
			if (CalendarDate{}).Overlaps(master, windowStart, windowEnd) {
				output = append(output, recurrence.marked(master, master["seriesId"], false, false))
			}
			continue
		}
		replacements := bySeries[RecordValues{}.String(master["id"])]
		for _, instance := range recurrence.instances(master, rule, windowStart, windowEnd) {
			replacement, replaced := replacements[RecordValues{}.String(instance["recurrenceId"])]
			if replaced {
				if replacement["status"] != "cancelled" && (CalendarDate{}).Overlaps(replacement, windowStart, windowEnd) {
					output = append(output, recurrence.marked(replacement, master["id"], true, true))
				}
			} else if (CalendarDate{}).Overlaps(instance, windowStart, windowEnd) {
				output = append(output, instance)
			}
			if len(output) > recurrence.mostOccurrences() {
				return nil, fmt.Sprintf("calendar range exceeds %d occurrences", recurrence.mostOccurrences())
			}
		}
	}
	for _, candidate := range exceptions {
		if known[(RecordValues{}).String(candidate["seriesId"])] {
			continue
		}
		if candidate["status"] != "cancelled" && (CalendarDate{}).Overlaps(candidate, windowStart, windowEnd) {
			output = append(output, recurrence.marked(candidate, candidate["seriesId"], true, true))
		}
	}
	sort.SliceStable(output, func(left, right int) bool {
		return recurrence.compare(output[left], output[right]) < 0
	})
	return output, ""
}

// compare orders events by start, then by id.
func (CalendarRecurrence) compare(left map[string]any, right map[string]any) float64 {
	difference := EpochTime{}.Parse(RecordValues{}.String(left["start"])) - EpochTime{}.Parse(RecordValues{}.String(right["start"]))
	if difference != 0 && !math.IsNaN(difference) {
		return difference
	}
	return float64(TextOrder{}.Compare(RecordValues{}.String(left["id"]), RecordValues{}.String(right["id"])))
}

// instances answers the occurrences of a recurring master near a window.
func (recurrence CalendarRecurrence) instances(master map[string]any, rule string, windowStart float64, windowEnd float64) []map[string]any {
	padding := recurrence.dayMilliseconds() * 2
	from, to := int64(windowStart-padding), int64(windowEnd+padding)
	if (RecordValues{}).Truthy(master["allDay"]) && (CalendarDate{}).IsDate(master["start"]) && (CalendarDate{}).IsDate(master["end"]) {
		return recurrence.allDayInstances(master, rule, from, to)
	}
	zone := "UTC"
	if (CalendarZone{}).Valid(master["timeZone"]) {
		zone = master["timeZone"].(string)
	}
	sourceStart := int64(EpochTime{}.Parse(RecordValues{}.String(master["start"])))
	duration := int64(math.Max(0, EpochTime{}.Parse(RecordValues{}.String(RecordValues{}.Or(master["end"], master["start"])))-float64(sourceStart)))
	floatingStart := CalendarZone{}.ToFloating(sourceStart, zone)
	order := []string{}
	actual := map[string]int64{}
	add := func(instant int64) {
		key := EpochTime{}.Text(instant)
		if _, seen := actual[key]; !seen {
			order = append(order, key)
		}
		actual[key] = instant
	}
	for _, floating := range (RecurrenceRule{}).Parsed(rule, floatingStart).Between(from, to) {
		add(CalendarZone{}.FromFloating(floating, zone))
	}
	for _, value := range recurrence.dateList(master["recurrenceDates"], false) {
		add(int64(EpochTime{}.Parse(value)))
	}
	excluded := map[string]bool{}
	for _, value := range recurrence.dateList(master["exceptionDates"], false) {
		excluded[EpochTime{}.Text(int64(EpochTime{}.Parse(value)))] = true
	}
	kept := []int64{}
	for _, key := range order {
		if !excluded[key] {
			kept = append(kept, actual[key])
		}
	}
	sort.SliceStable(kept, func(left, right int) bool { return kept[left] < kept[right] })
	instances := []map[string]any{}
	for _, instant := range kept {
		recurrenceId := EpochTime{}.Text(instant)
		instances = append(instances, recurrence.occurrence(master, rule, recurrenceId, recurrenceId, EpochTime{}.Text(instant+duration)))
	}
	return instances
}

func (recurrence CalendarRecurrence) allDayInstances(master map[string]any, rule string, from int64, to int64) []map[string]any {
	days := int(math.Max(1, float64(CalendarDate{}.DaysBetween(master["start"].(string), master["end"].(string)))))
	start := int64(EpochTime{}.Parse(master["start"].(string) + "T00:00:00.000Z"))
	order := []string{}
	seen := map[string]bool{}
	add := func(date string) {
		if !seen[date] {
			seen[date] = true
			order = append(order, date)
		}
	}
	for _, instant := range (RecurrenceRule{}).Parsed(rule, start).Between(from, to) {
		add(EpochTime{}.Text(instant)[:len("2006-01-02")])
	}
	for _, value := range recurrence.dateList(master["recurrenceDates"], true) {
		add(value)
	}
	excluded := map[string]bool{}
	for _, value := range recurrence.dateList(master["exceptionDates"], true) {
		excluded[value] = true
	}
	kept := []string{}
	for _, date := range order {
		if !excluded[date] {
			kept = append(kept, date)
		}
	}
	sort.Strings(kept)
	instances := []map[string]any{}
	for _, date := range kept {
		instance := recurrence.occurrence(master, rule, date, date, CalendarDate{}.AddDays(date, days))
		instance["allDay"] = true
		instances = append(instances, instance)
	}
	return instances
}

// occurrence answers one generated instance of a master.
func (recurrence CalendarRecurrence) occurrence(master map[string]any, rule string, recurrenceId string, start string, end string) map[string]any {
	instance := StoredRecords{}.Without(master)
	instance["id"] = RecordValues{}.String(master["id"]) + "::" + recurrence.encoded(recurrenceId)
	instance["seriesId"] = master["id"]
	instance["recurrenceId"] = recurrenceId
	instance["start"] = start
	instance["end"] = end
	instance["recurrence"] = rule
	instance["isRecurring"] = true
	instance["isException"] = false
	return instance
}

// marked answers a copy of an event as an ordinary event or an exception
// of a series.
func (CalendarRecurrence) marked(event map[string]any, seriesId any, recurring bool, exception bool) map[string]any {
	copied := StoredRecords{}.Without(event)
	if exception {
		copied["seriesId"] = seriesId
	}
	copied["isRecurring"] = recurring
	copied["isException"] = exception
	return copied
}

// groupExceptions indexes exceptions by series, under both the date and the
// instant form of their recurrence id when each can be read.
func (CalendarRecurrence) groupExceptions(exceptions []map[string]any) map[string]map[string]map[string]any {
	grouped := map[string]map[string]map[string]any{}
	for _, exception := range exceptions {
		date := CalendarDate{}.Coerce(exception["recurrenceId"])
		instant := CalendarDate{}.NormalizeOccurrence(exception["recurrenceId"], false)
		series := RecordValues{}.String(exception["seriesId"])
		if date == "" && instant == "" {
			continue
		}
		if grouped[series] == nil {
			grouped[series] = map[string]map[string]any{}
		}
		if date != "" {
			grouped[series][date] = exception
		}
		if instant != "" {
			grouped[series][instant] = exception
		}
	}
	return grouped
}

// dateList answers the first 256 usable entries of a stored date list.
func (CalendarRecurrence) dateList(input any, allDay bool) []string {
	values, isList := RecordValues{}.List(input)
	if !isList {
		return []string{}
	}
	if len(values) > 256 {
		values = values[:256]
	}
	dates := []string{}
	for _, value := range values {
		if allDay {
			if date := (CalendarDate{}).Coerce(value); date != "" {
				dates = append(dates, date)
			}
		} else if text, isText := value.(string); isText && (CalendarDate{}).IsDateTime(text) {
			dates = append(dates, text)
		}
	}
	return dates
}

// encoded answers text as encodeURIComponent wrote it.
func (CalendarRecurrence) encoded(text string) string {
	escaped := url.QueryEscape(text)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	for _, kept := range []string{"!", "'", "(", ")", "*"} {
		escaped = strings.ReplaceAll(escaped, url.QueryEscape(kept), kept)
	}
	return escaped
}

func (CalendarRecurrence) bounded(parts map[string]string, key string, low int, high int) bool {
	value, present := parts[key]
	if !present {
		return true
	}
	if !regexp.MustCompile(`^\d+$`).MatchString(value) {
		return false
	}
	number, err := strconv.Atoi(value)
	return err == nil && number >= low && number <= high
}

func (CalendarRecurrence) listMatches(value string, pattern *regexp.Regexp) bool {
	items := strings.Split(value, ",")
	if value == "" || len(items) > 64 {
		return false
	}
	for _, item := range items {
		if !pattern.MatchString(item) {
			return false
		}
	}
	return true
}

func (CalendarRecurrence) integers(value string, low int, high int, zero bool) bool {
	items := strings.Split(value, ",")
	if value == "" || len(items) > 64 {
		return false
	}
	for _, item := range items {
		if !regexp.MustCompile(`^[-+]?\d+$`).MatchString(item) {
			return false
		}
		number, err := strconv.Atoi(strings.TrimPrefix(item, "+"))
		if err != nil || number < low || number > high || (!zero && number == 0) {
			return false
		}
	}
	return true
}

func (recurrence CalendarRecurrence) allowed(key string) bool {
	for _, candidate := range recurrence.keys() {
		if candidate == key {
			return true
		}
	}
	return false
}

func (CalendarRecurrence) keys() []string {
	return []string{"FREQ", "INTERVAL", "COUNT", "UNTIL", "WKST", "BYDAY", "BYMONTHDAY", "BYYEARDAY", "BYWEEKNO", "BYMONTH", "BYSETPOS"}
}

func (CalendarRecurrence) longestRule() int { return 1024 }

func (CalendarRecurrence) largestCount() int { return 10_000 }

func (CalendarRecurrence) largestInterval() int { return 365 }

func (CalendarRecurrence) longestRangeDays() int { return 400 }

func (CalendarRecurrence) mostOccurrences() int { return 2_000 }

func (CalendarRecurrence) dayMilliseconds() float64 { return 24 * 60 * 60 * 1000 }
