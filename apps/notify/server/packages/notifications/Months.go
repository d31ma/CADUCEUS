package notifications

import "time"

type Months struct{}

func (months Months) Recent(now time.Time) []string {
	current := months.firstDay(now.UTC())
	previous := current.AddDate(0, -1, 0)
	return []string{current.Format(months.layout()), previous.Format(months.layout())}
}

func (Months) layout() string {
	return "200601"
}

func (Months) firstDay(moment time.Time) time.Time {
	return time.Date(moment.Year(), moment.Month(), 1, 0, 0, 0, 0, time.UTC)
}
