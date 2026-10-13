import (
	"math"
	"net/mail"
	"strings"
	"time"
)

// EpochTime reads and writes instants as JavaScript did: milliseconds since
// the epoch, ISO text from toISOString(), and Date.parse() answering NaN for
// text that is not a date.
type EpochTime struct{}

// Now answers Date.now().
func (EpochTime) Now() int64 {
	return time.Now().UnixMilli()
}

// NowText answers new Date().toISOString().
func (moment EpochTime) NowText() string {
	return moment.Text(moment.Now())
}

// Text answers new Date(milliseconds).toISOString().
func (EpochTime) Text(milliseconds int64) string {
	return time.UnixMilli(milliseconds).UTC().Format("2006-01-02T15:04:05.000Z")
}

// Parse answers Date.parse(text): milliseconds, or NaN.
func (moment EpochTime) Parse(text string) float64 {
	value := strings.TrimSpace(text)
	for _, layout := range moment.utcLayouts() {
		if parsed, err := time.Parse(layout, value); err == nil {
			return float64(parsed.UnixMilli())
		}
	}
	for _, layout := range moment.localLayouts() {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return float64(parsed.UnixMilli())
		}
	}
	if parsed, err := mail.ParseDate(value); err == nil {
		return float64(parsed.UnixMilli())
	}
	return math.NaN()
}

// utcLayouts are the ISO forms JavaScript reads with an offset, and the
// date-only forms it reads as UTC.
func (EpochTime) utcLayouts() []string {
	return []string{time.RFC3339Nano, "2006-01-02T15:04Z07:00", "2006-01-02", "2006-01", "2006"}
}

// localLayouts are the ISO date-time forms without an offset, which
// JavaScript reads in local time.
func (EpochTime) localLayouts() []string {
	return []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04"}
}
