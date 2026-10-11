import (
	"errors"
	"math"
	"strings"
	"time"
)

// CalendarDateUtils is the Go port of client/shared/scripts/calendar-date.js.
// All-day events carry civil dates ("2026-10-10") and an exclusive end date;
// timed events carry instants. Every local reading takes the browser's time
// zone as a *time.Location, because a WebAssembly service has no local zone
// of its own that follows daylight saving time.
type CalendarDateUtils struct{}

// IsCalendarDate answers whether value is a real civil date in YYYY-MM-DD form.
func (dates CalendarDateUtils) IsCalendarDate(value string) bool {
	_, failure := dates.civil(value)
	return failure == nil
}

// AddCalendarDays moves a civil date by whole days.
func (dates CalendarDateUtils) AddCalendarDays(value string, days int) (string, error) {
	day, failure := dates.civil(value)
	return day.AddDate(0, 0, days).Format(dates.dateLayout()), failure
}

// CalendarDaysBetween counts the whole days from start to end.
func (dates CalendarDateUtils) CalendarDaysBetween(start string, end string) (int, error) {
	first, startFailure := dates.civil(start)
	last, endFailure := dates.civil(end)
	days := int(math.Round(last.Sub(first).Hours() / dates.hoursPerDay()))
	return days, errors.Join(startFailure, endFailure)
}

// AllDayRangeForDate is the one-day all-day range that starts on date.
func (dates CalendarDateUtils) AllDayRangeForDate(date string) (EventRange, error) {
	end, failure := dates.AddCalendarDays(date, 1)
	return EventRange{Start: date, End: end, AllDay: true}, failure
}

// ShiftAllDayRangeToDate keeps an all-day range's length and starts it on target.
func (dates CalendarDateUtils) ShiftAllDayRangeToDate(span EventRange, target string) (EventRange, error) {
	start := dates.CalendarDateValue(span.Start)
	end := dates.CalendarDateValue(span.End)
	length, lengthFailure := dates.CalendarDaysBetween(start, end)
	shifted, endFailure := dates.AddCalendarDays(target, max(1, length))
	return EventRange{Start: target, End: shifted, AllDay: true}, errors.Join(lengthFailure, endFailure)
}

// MoveTimedRangeToDate keeps a timed range's wall-clock start and duration and
// moves it to the target civil date.
func (dates CalendarDateUtils) MoveTimedRangeToDate(span EventRange, target string, zone *time.Location) (EventRange, error) {
	start, startFound := dates.ParseInstant(span.Start, zone)
	end, endFound := dates.ParseInstant(TextUtils{}.FirstOf(span.End, span.Start), zone)
	day, dayFailure := dates.ParseLocalDate(target, zone)
	local := start.In(zone)
	moved := time.Date(day.Year(), day.Month(), day.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), zone)
	length := max(0, end.Sub(start))
	next := EventRange{Start: dates.InstantText(moved), End: dates.InstantText(moved.Add(length)), AllDay: false}
	return next, errors.Join(dayFailure, dates.instantFailure(startFound, endFound))
}

// TimedRangeForAllDayDates turns an exclusive all-day range into editable
// 09:00 to 10:00 wall-clock values on its first and last day.
func (dates CalendarDateUtils) TimedRangeForAllDayDates(span EventRange, zone *time.Location) (EventRange, error) {
	startDate := dates.CalendarDateValue(span.Start)
	endDate := dates.CalendarDateValue(span.End)
	length, lengthFailure := dates.CalendarDaysBetween(startDate, endDate)
	lastDate, lastFailure := dates.AddCalendarDays(endDate, -1)
	first, _ := dates.ParseLocalDate(startDate, zone)
	last, _ := dates.ParseLocalDate(lastDate, zone)
	start := time.Date(first.Year(), first.Month(), first.Day(), dates.morningHour(), 0, 0, 0, zone)
	end := time.Date(last.Year(), last.Month(), last.Day(), dates.morningHour()+1, 0, 0, 0, zone)
	next := EventRange{Start: dates.LocalDateTimeValue(start), End: dates.LocalDateTimeValue(end), AllDay: false}
	return next, errors.Join(lengthFailure, lastFailure, dates.lengthFailure(length))
}

// EventCoversCalendarDate answers whether an event shows on a civil date: an
// all-day event on each day of its range, a timed event on its start day.
func (dates CalendarDateUtils) EventCoversCalendarDate(event CalendarEvent, date string, zone *time.Location) bool {
	coverage := map[bool]func() bool{
		true:  func() bool { return dates.allDayCovers(event, date) },
		false: func() bool { return dates.timedCovers(event, date, zone) },
	}
	return dates.validCoverage(dates.IsCalendarDate(date), coverage[event.AllDay])
}

// CalendarCursorForEvent is the instant that reveals event in another view.
func (dates CalendarDateUtils) CalendarCursorForEvent(event CalendarEvent, zone *time.Location) (time.Time, bool) {
	cursors := map[bool]func() (time.Time, bool){
		true:  func() (time.Time, bool) { return dates.allDayCursor(event, zone) },
		false: func() (time.Time, bool) { return dates.ParseInstant(event.Start, zone) },
	}
	return cursors[event.AllDay]()
}

// CalendarDateValue reads the civil date of a stored value. Legacy all-day
// rows were saved at UTC midnight, so the date part of any instant is kept.
func (dates CalendarDateUtils) CalendarDateValue(value string) string {
	_, parsed := dates.ParseInstant(value, time.UTC)
	return dates.dateValue(value, dates.IsCalendarDate(value), parsed)
}

// CivilDate is the civil date of moment in its own location.
func (dates CalendarDateUtils) CivilDate(moment time.Time) string {
	return moment.Format(dates.dateLayout())
}

// LocalDateTimeValue is moment as an <input type="datetime-local"> value.
func (dates CalendarDateUtils) LocalDateTimeValue(moment time.Time) string {
	return moment.Format(dates.minuteLayout())
}

// ParseLocalDate is noon of a civil date in zone, which no time zone change moves to another day.
func (dates CalendarDateUtils) ParseLocalDate(value string, zone *time.Location) (time.Time, error) {
	day, failure := dates.civil(value)
	return time.Date(day.Year(), day.Month(), day.Day(), dates.noonHour(), 0, 0, 0, zone), failure
}

// ParseInstant reads a value the way JavaScript's Date constructor reads the
// forms the calendar uses: a bare date is UTC midnight, a date and time with
// no offset is local wall-clock time, and an ISO instant keeps its offset.
func (dates CalendarDateUtils) ParseInstant(value string, zone *time.Location) (time.Time, bool) {
	layout := dates.instantLayout(len(value), dates.zoned(value))
	moment, failure := time.ParseInLocation(layout, value, dates.readingZone(len(value), zone))
	return moment, failure == nil
}

// InstantText is moment as JavaScript's toISOString writes it.
func (dates CalendarDateUtils) InstantText(moment time.Time) string {
	return moment.UTC().Format(dates.instantTextLayout())
}

func (dates CalendarDateUtils) civil(value string) (time.Time, error) {
	day, failure := time.Parse(dates.dateLayout(), value)
	return day, errors.Join(failure, dates.dateLengthFailure(len(value)))
}

func (dates CalendarDateUtils) dateLengthFailure(length int) error {
	if length != len(dates.dateLayout()) {
		return errors.New("calendar date is required")
	}
	return nil
}

func (CalendarDateUtils) instantFailure(startFound bool, endFound bool) error {
	if !startFound || !endFound {
		return errors.New("valid timed range is required")
	}
	return nil
}

func (CalendarDateUtils) lengthFailure(length int) error {
	if length < 1 {
		return errors.New("valid all-day range is required")
	}
	return nil
}

func (CalendarDateUtils) validCoverage(valid bool, covered func() bool) bool {
	if valid && covered() {
		return true
	}
	return false
}

func (dates CalendarDateUtils) allDayCovers(event CalendarEvent, date string) bool {
	start := dates.CalendarDateValue(event.Start)
	end := dates.CalendarDateValue(TextUtils{}.FirstOf(event.End, event.Start))
	if start != "" && end != "" && start <= date && date < end {
		return true
	}
	return false
}

func (dates CalendarDateUtils) timedCovers(event CalendarEvent, date string, zone *time.Location) bool {
	start, found := dates.ParseInstant(event.Start, zone)
	if found && dates.CivilDate(start.In(zone)) == date {
		return true
	}
	return false
}

func (dates CalendarDateUtils) allDayCursor(event CalendarEvent, zone *time.Location) (time.Time, bool) {
	day, failure := dates.ParseLocalDate(dates.CalendarDateValue(event.Start), zone)
	return day, failure == nil
}

func (CalendarDateUtils) dateValue(value string, civil bool, parsed bool) string {
	switch {
	case civil:
		return value
	case parsed:
		return value[:10]
	default:
		return ""
	}
}

func (CalendarDateUtils) zoned(value string) bool {
	if strings.HasSuffix(value, "Z") || strings.LastIndexAny(value, "+-") > strings.Index(value, "T") {
		return true
	}
	return false
}

func (dates CalendarDateUtils) instantLayout(length int, zoned bool) string {
	switch {
	case length == len(dates.dateLayout()):
		return dates.dateLayout()
	case zoned:
		return time.RFC3339Nano
	case length == len(dates.minuteLayout()):
		return dates.minuteLayout()
	default:
		return "2006-01-02T15:04:05.999999999"
	}
}

func (dates CalendarDateUtils) readingZone(length int, zone *time.Location) *time.Location {
	if length == len(dates.dateLayout()) {
		return time.UTC
	}
	return zone
}

func (CalendarDateUtils) dateLayout() string { return "2006-01-02" }

func (CalendarDateUtils) minuteLayout() string { return "2006-01-02T15:04" }

func (CalendarDateUtils) instantTextLayout() string { return "2006-01-02T15:04:05.000Z" }

func (CalendarDateUtils) hoursPerDay() float64 { return 24 }

func (CalendarDateUtils) morningHour() int { return 9 }

func (CalendarDateUtils) noonHour() int { return 12 }
