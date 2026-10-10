import "time"

// DateLabelUtils writes the date labels that formatPreferenceDate gave the
// calendar through Intl.DateTimeFormat for the English interface locale. A
// WebAssembly service has no Intl, so each label is a fixed Go layout that
// matches Chromium's en-US output, including the 12 or 24 hour clock choice.
type DateLabelUtils struct{}

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
	return moment.Format("Jan 2")
}

// ShortMonthDayYear is { month: 'short', day: 'numeric', year: 'numeric' }: "Oct 10, 2026".
func (labels DateLabelUtils) ShortMonthDayYear(moment time.Time) string {
	return moment.Format("Jan 2, 2006")
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

func (DateLabelUtils) clockLayouts() map[bool]string {
	return map[bool]string{false: "03:04 PM", true: "15:04"}
}
