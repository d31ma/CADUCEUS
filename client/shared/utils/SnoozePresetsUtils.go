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
	return DateLabelUtils{}.ShortMonthDay(moment) + ", " + clock
}

func (SnoozePresetsUtils) label(failure error, when string) string {
	if failure != nil {
		return "Snoozed"
	}
	return "Snoozed until " + when
}

func (SnoozePresetsUtils) day() time.Duration { return 24 * time.Hour }

// Preset is the local moment a reader snooze preset names, as Mailspring's
// DateUtils (snooze-presets.js) chose it. An unknown preset is the zero time.
func (SnoozePresetsUtils) Preset(id string, now time.Time) time.Time {
	const evening, morning, later, week = 20, 9, 3, 7
	weekday := int(now.Weekday())
	at := func(days int, hour int) func() time.Time {
		return func() time.Time {
			return time.Date(now.Year(), now.Month(), now.Day()+days, hour, 0, 0, 0, now.Location())
		}
	}
	presets := map[string]func() time.Time{
		"later-today":  at(0, now.Hour()+later),
		"tonight":      at(now.Hour()/evening, evening*(1-now.Hour()/evening)),
		"tomorrow":     at(1, morning),
		"this-weekend": at(week-(weekday+1)%week, morning),
		"next-week":    at((week-weekday)%week+1, morning),
		"next-month":   func() time.Time { return time.Date(now.Year(), now.Month()+1, 1, morning, 0, 0, 0, now.Location()) },
	}
	chosen, found := presets[id]
	return map[bool]func() time.Time{true: chosen, false: func() time.Time { return time.Time{} }}[found]()
}

// Custom is a datetime-local value as a local moment, or the zero time.
func (SnoozePresetsUtils) Custom(value string, now time.Time) time.Time {
	moment, _ := time.ParseInLocation("2006-01-02T15:04", value, now.Location())
	return moment
}

// Timestamp is a moment as the API takes it: UTC ISO text with milliseconds.
func (SnoozePresetsUtils) Timestamp(moment time.Time) string {
	return moment.UTC().Format("2006-01-02T15:04:05.000Z")
}
