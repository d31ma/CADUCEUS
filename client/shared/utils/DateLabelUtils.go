import (
	"fmt"
	"time"
)

// DateLabelUtils writes the date and time labels the mailbox views show. It
// matches what Intl.DateTimeFormat gives for the English interface locale,
// in the browser's time zone: a WebAssembly service has no Intl.
type DateLabelUtils struct{}

// Numeric is Intl.DateTimeFormat('en').format(value): "10/11/2026", or ""
// when the value is not an ISO date.
func (DateLabelUtils) Numeric(value string) string {
	moment, failure := time.Parse(time.RFC3339Nano, value)
	return DateLabelUtils{}.numericUnless(moment.Local(), failure)
}

// SnoozedUntil is the snooze badge: the time alone within a day, the month
// and day before it otherwise.
func (DateLabelUtils) SnoozedUntil(value string, now time.Time, use24HourClock bool) string {
	moment, failure := time.Parse(time.RFC3339Nano, value)
	local := moment.Local()
	clock := DateLabelUtils{}.clock(local, use24HourClock)
	label := DateLabelUtils{}.withDay(local, clock, local.Sub(now) < DateLabelUtils{}.day())
	return DateLabelUtils{}.snoozeText(label, failure)
}

// MonthDay is a calendar day such as "2026-01-05" as "Jan 5", read in UTC.
func (DateLabelUtils) MonthDay(day string) string {
	moment, failure := time.Parse("2006-01-02", day)
	return DateLabelUtils{}.monthDayUnless(moment, failure)
}

func (DateLabelUtils) numericUnless(moment time.Time, failure error) string {
	if failure != nil {
		return ""
	}
	return fmt.Sprintf("%d/%d/%d", int(moment.Month()), moment.Day(), moment.Year())
}

func (DateLabelUtils) monthDayUnless(moment time.Time, failure error) string {
	if failure != nil {
		return ""
	}
	return moment.Format(DateLabelUtils{}.monthDayLayout())
}

func (DateLabelUtils) clock(moment time.Time, use24HourClock bool) string {
	minutes := []string{"", fmt.Sprintf(":%02d", moment.Minute())}[min(moment.Minute(), 1)]
	if use24HourClock {
		return fmt.Sprintf("%02d%s", moment.Hour(), minutes)
	}
	half := DateLabelUtils{}.halfDay()
	return fmt.Sprintf("%d%s %s", (moment.Hour()+half-1)%half+1, minutes, []string{"AM", "PM"}[moment.Hour()/half])
}

func (DateLabelUtils) withDay(moment time.Time, clock string, soon bool) string {
	if soon {
		return clock
	}
	return moment.Format(DateLabelUtils{}.monthDayLayout()) + ", " + clock
}

func (DateLabelUtils) snoozeText(label string, failure error) string {
	if failure != nil {
		return "Snoozed"
	}
	return "Snoozed until " + label
}

func (DateLabelUtils) day() time.Duration { return 24 * time.Hour }

func (DateLabelUtils) halfDay() int { return 12 }

func (DateLabelUtils) monthDayLayout() string { return "Jan 2" }
