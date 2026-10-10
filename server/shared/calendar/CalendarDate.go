import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CalendarDate handles RFC 5545 DATE values. A DATE is a civil day kept as
// YYYY-MM-DD, never an instant, so time zones and daylight saving cannot move
// an all-day event to another day. All-day end dates are exclusive.
type CalendarDate struct{}

// IsDate reports whether value is a real calendar day written YYYY-MM-DD.
func (date CalendarDate) IsDate(value any) bool {
	text, isText := value.(string)
	parts := date.pattern().FindStringSubmatch(text)
	if !isText || parts == nil {
		return false
	}
	year, _ := strconv.Atoi(parts[1])
	month, _ := strconv.Atoi(parts[2])
	day, _ := strconv.Atoi(parts[3])
	instant := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	// Date.UTC reads the years 0 to 99 as 1900 to 1999, so those never
	// matched in JavaScript.
	return year >= date.firstFullYear() && instant.Year() == year && int(instant.Month()) == month && instant.Day() == day
}

// IsDateTime reports whether value is date-time text that parses.
func (CalendarDate) IsDateTime(value any) bool {
	text, isText := value.(string)
	return isText && strings.Contains(text, "T") && !math.IsNaN(EpochTime{}.Parse(text))
}

// Coerce answers the calendar day of a date or date-time value, or "".
func (date CalendarDate) Coerce(value any) string {
	if date.IsDate(value) {
		return value.(string)
	}
	if date.IsDateTime(value) {
		return value.(string)[:len("2006-01-02")]
	}
	return ""
}

// Overlaps reports whether an event overlaps the window between two instants
// (milliseconds; the ends may be infinite).
func (date CalendarDate) Overlaps(event map[string]any, rangeStart float64, rangeEnd float64) bool {
	if (RecordValues{}).Truthy(event["allDay"]) && date.IsDate(event["start"]) && date.IsDate(event["end"]) {
		start := date.epoch(event["start"].(string))
		exclusiveEnd := date.epoch(event["end"].(string))
		return exclusiveEnd > rangeStart && start <= rangeEnd
	}
	start := EpochTime{}.Parse(RecordValues{}.Text(event["start"]))
	end := EpochTime{}.Parse(RecordValues{}.Text(RecordValues{}.Or(event["end"], event["start"])))
	return !math.IsNaN(start) && !math.IsNaN(end) && end >= rangeStart && start <= rangeEnd
}

func (date CalendarDate) epoch(value string) float64 {
	parsed, _ := time.Parse("2006-01-02", value)
	return float64(parsed.UnixMilli())
}

func (CalendarDate) pattern() *regexp.Regexp {
	return regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
}

func (CalendarDate) firstFullYear() int { return 100 }
