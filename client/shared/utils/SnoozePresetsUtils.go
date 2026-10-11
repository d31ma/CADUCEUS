import "time"

// SnoozePresetsUtils holds the snooze wording shared by the list and the
// reader (snooze-presets.js). The list needs only the badge; the reader's
// presets join it at merge.
type SnoozePresetsUtils struct{}

// UntilLabel is the snooze badge: "Snoozed until 9 AM" within a day,
// "Snoozed until Oct 11, 9 AM" after, and "Snoozed" for a value that is not
// a date.
func (SnoozePresetsUtils) UntilLabel(value string, now time.Time, use24HourClock bool) string {
	moment, failure := time.Parse(time.RFC3339Nano, value)
	local := moment.Local()
	clock := DateLabelUtils{}.Clock(local, use24HourClock)
	return SnoozePresetsUtils{}.label(failure, SnoozePresetsUtils{}.withDay(local, clock, local.Sub(now) < SnoozePresetsUtils{}.day()))
}

func (SnoozePresetsUtils) withDay(moment time.Time, clock string, soon bool) string {
	if soon {
		return clock
	}
	return DateLabelUtils{}.MonthDayOf(moment) + ", " + clock
}

func (SnoozePresetsUtils) label(failure error, when string) string {
	if failure != nil {
		return "Snoozed"
	}
	return "Snoozed until " + when
}

func (SnoozePresetsUtils) day() time.Duration { return 24 * time.Hour }
