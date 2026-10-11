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

// AddDays answers the calendar day a whole number of days after a date.
func (date CalendarDate) AddDays(value string, days int) string {
	return EpochTime{}.Text(int64(date.epoch(value)) + int64(days)*date.dayMilliseconds())[:len("2006-01-02")]
}

// DaysBetween answers how many days lie from one calendar day to another.
func (date CalendarDate) DaysBetween(start string, end string) int {
	return int(math.Round((date.epoch(end) - date.epoch(start)) / float64(date.dayMilliseconds())))
}

// ValidateRange answers the trimmed start and end of an event, or the
// problem with them. All-day events take dates and an exclusive end; timed
// events take date-times.
func (date CalendarDate) ValidateRange(rawStart any, rawEnd any, allDay bool) (string, string, string) {
	start := Security{}.Trim(RecordValues{}.Text(rawStart))
	end := Security{}.Trim(RecordValues{}.Text(rawEnd))
	if allDay {
		if !date.IsDate(start) || !date.IsDate(end) {
			return "", "", "all-day start and end must be date-only YYYY-MM-DD values"
		}
		if end <= start {
			return "", "", "all-day end must be after start"
		}
		return start, end, ""
	}
	if !date.IsDateTime(start) {
		return "", "", "valid start (ISO 8601 date-time) is required"
	}
	if !date.IsDateTime(end) {
		return "", "", "valid end (ISO 8601 date-time) is required"
	}
	if (EpochTime{}).Parse(end) < (EpochTime{}).Parse(start) {
		return "", "", "end must be on or after start"
	}
	return start, end, ""
}

// FromCompact answers an RFC 5545 YYYYMMDD date as YYYY-MM-DD; other text
// is answered as it is.
func (CalendarDate) FromCompact(value string) string {
	if !regexp.MustCompile(`^\d{8}$`).MatchString(value) {
		return value
	}
	return value[0:4] + "-" + value[4:6] + "-" + value[6:8]
}

// NormalizeOccurrence answers the identity of one occurrence: the date of
// an all-day occurrence, or the instant of a timed one as ISO text.
func (date CalendarDate) NormalizeOccurrence(value any, allDay bool) string {
	if allDay {
		if date.IsDate(value) {
			return value.(string)
		}
		return ""
	}
	if !date.IsDateTime(value) {
		return ""
	}
	return EpochTime{}.Text(int64(EpochTime{}.Parse(value.(string))))
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

// IsPast reports whether an event has ended: an all-day event once its
// exclusive end day has come in its time zone, a timed event once its end
// instant has gone.
func (date CalendarDate) IsPast(event map[string]any, now int64) bool {
	if (RecordValues{}).Truthy(event["allDay"]) && date.IsDate(event["end"]) {
		zone := RecordValues{}.Text(RecordValues{}.Or(event["timeZone"], "UTC"))
		return event["end"].(string) <= date.TodayInTimeZone(zone, now)
	}
	end := EpochTime{}.Parse(RecordValues{}.String(RecordValues{}.Or(event["end"], event["start"])))
	return !math.IsNaN(end) && end < float64(now)
}

// TodayInTimeZone answers the calendar day it is at an instant in a time
// zone; a zone that is not valid reads as UTC.
func (CalendarDate) TodayInTimeZone(zone string, now int64) string {
	location := CalendarZone{}.Location(zone)
	if location == nil {
		location = time.UTC
	}
	return time.UnixMilli(now).In(location).Format("2006-01-02")
}

func (date CalendarDate) epoch(value string) float64 {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return math.NaN()
	}
	return float64(parsed.UnixMilli())
}

func (CalendarDate) dayMilliseconds() int64 { return 24 * 60 * 60 * 1000 }

func (CalendarDate) pattern() *regexp.Regexp {
	return regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
}

func (CalendarDate) firstFullYear() int { return 100 }
