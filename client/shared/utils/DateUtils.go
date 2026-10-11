import "time"

// DateUtils writes timestamps for people. It replaces the Intl date format
// of shared/scripts/localization.js with a fixed English form, because Go
// services have no Intl.
type DateUtils struct{}

// Label is "Jan 5, 2026, 2:30 PM" in local time, or empty for no date.
func (DateUtils) Label(value string, now time.Time) string {
	moment, failure := time.Parse(time.RFC3339Nano, value)
	return map[bool]string{true: moment.In(now.Location()).Format("Jan 2, 2006, 3:04 PM")}[failure == nil]
}
