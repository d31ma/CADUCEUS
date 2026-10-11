import (
	"fmt"
	"time"
)

// DateLabelUtils writes the date and time labels the mailbox views and the
// calendar show. Each matches what Intl.DateTimeFormat gives for the English
// interface locale (Chromium's en-US output), in the browser's time zone,
// including the 12 or 24 hour clock choice: a WebAssembly service has no Intl.
type DateLabelUtils struct{}

// Numeric is Intl.DateTimeFormat('en').format(value): "10/11/2026", or ""
// when the value is not an ISO date.
func (labels DateLabelUtils) Numeric(value string) string {
	moment, failure := time.Parse(time.RFC3339Nano, value)
	return labels.formatUnless(moment.Local(), failure, "1/2/2006")
}

// MonthDay is a calendar day such as "2026-01-05" as "Jan 5", read in UTC.
func (labels DateLabelUtils) MonthDay(day string) string {
	moment, failure := time.Parse("2006-01-02", day)
	return labels.formatUnless(moment, failure, labels.monthDayLayout())
}

// Stamp is a stored timestamp as "Jan 5, 2026, 2:30 PM" in the location of
// now, or "" when the value is not an ISO date.
func (labels DateLabelUtils) Stamp(value string, now time.Time) string {
	moment, failure := time.Parse(time.RFC3339Nano, value)
	return labels.formatUnless(moment.In(now.Location()), failure, labels.stampLayout())
}

// StoredDate is a stored timestamp as "Oct 10, 2026" at the browser's time
// zone offset, in minutes east of UTC, or "" when it is not an ISO date.
func (labels DateLabelUtils) StoredDate(value string, offsetMinutes float64) string {
	return labels.atOffset(value, offsetMinutes, labels.dateLayout())
}

// StoredDateTime is a stored timestamp as "Oct 10, 2026, 3:04 PM" at the
// browser's time zone offset, or "" when it is not an ISO date.
func (labels DateLabelUtils) StoredDateTime(value string, offsetMinutes float64) string {
	return labels.atOffset(value, offsetMinutes, labels.stampLayout())
}

// Clock is {hour: 'numeric', minute: '2-digit'} with the minutes left out
// on the hour: "9 AM", "1:05 PM", or "09" and "13:05" on a 24-hour clock.
func (DateLabelUtils) Clock(moment time.Time, use24HourClock bool) string {
	minutes := []string{"", fmt.Sprintf(":%02d", moment.Minute())}[min(moment.Minute(), 1)]
	return DateLabelUtils{}.hours(moment, minutes, use24HourClock)
}

// WeekdayMonthDay is { weekday: 'long', month: 'long', day: 'numeric' }: "Saturday, October 10".
func (labels DateLabelUtils) WeekdayMonthDay(moment time.Time) string {
	return moment.Format("Monday, January 2")
}

// MonthYear is { month: 'long', year: 'numeric' }: "October 2026".
func (labels DateLabelUtils) MonthYear(moment time.Time) string {
	return moment.Format("January 2006")
}

// WeekdayMonthDayYear is the long day with its year: "Saturday, October 10, 2026".
func (labels DateLabelUtils) WeekdayMonthDayYear(moment time.Time) string {
	return moment.Format("Monday, January 2, 2006")
}

// ShortMonthDay is { month: 'short', day: 'numeric' }: "Oct 10".
func (labels DateLabelUtils) ShortMonthDay(moment time.Time) string {
	return moment.Format(labels.monthDayLayout())
}

// ShortMonthDayYear is { month: 'short', day: 'numeric', year: 'numeric' }: "Oct 10, 2026".
func (labels DateLabelUtils) ShortMonthDayYear(moment time.Time) string {
	return moment.Format(labels.dateLayout())
}

// YearAndDay is { day: 'numeric', year: 'numeric' }, which Chromium's en-US
// pattern writes as "2026 (day: 10)".
func (labels DateLabelUtils) YearAndDay(moment time.Time) string {
	return moment.Format("2006 (day: 2)")
}

// ShortWeekday is { weekday: 'short' }: "Sat".
func (labels DateLabelUtils) ShortWeekday(moment time.Time) string {
	return moment.Format("Mon")
}

// ShortWeekdayMonthDay is { weekday: 'short', month: 'short', day: 'numeric' }: "Sat, Oct 10".
func (labels DateLabelUtils) ShortWeekdayMonthDay(moment time.Time) string {
	return moment.Format("Mon, Jan 2")
}

// HourMinute is { hour: '2-digit', minute: '2-digit' }: "09:05 AM", or "09:05" on a 24 hour clock.
func (labels DateLabelUtils) HourMinute(moment time.Time, twentyFourHour bool) string {
	return moment.Format(labels.clockLayouts()[twentyFourHour])
}

// ShortDateTime is the short weekday, date and clock: "Sat, Oct 10, 2026, 09:05 AM".
func (labels DateLabelUtils) ShortDateTime(moment time.Time, twentyFourHour bool) string {
	return moment.Format("Mon, Jan 2, 2006, ") + labels.HourMinute(moment, twentyFourHour)
}

func (labels DateLabelUtils) atOffset(value string, offsetMinutes float64, layout string) string {
	moment, failure := time.Parse(time.RFC3339Nano, value)
	return labels.formatUnless(moment.In(time.FixedZone("", int(offsetMinutes*60))), failure, layout)
}

func (DateLabelUtils) formatUnless(moment time.Time, failure error, layout string) string {
	if failure != nil {
		return ""
	}
	return moment.Format(layout)
}

func (DateLabelUtils) hours(moment time.Time, minutes string, use24HourClock bool) string {
	if use24HourClock {
		return fmt.Sprintf("%02d%s", moment.Hour(), minutes)
	}
	half := DateLabelUtils{}.halfDay()
	return fmt.Sprintf("%d%s %s", (moment.Hour()+half-1)%half+1, minutes, []string{"AM", "PM"}[moment.Hour()/half])
}

func (DateLabelUtils) clockLayouts() map[bool]string {
	return map[bool]string{false: "03:04 PM", true: "15:04"}
}

func (DateLabelUtils) halfDay() int { return 12 }

func (DateLabelUtils) monthDayLayout() string { return "Jan 2" }

func (DateLabelUtils) dateLayout() string { return "Jan 2, 2006" }

func (DateLabelUtils) stampLayout() string { return "Jan 2, 2006, 3:04 PM" }
