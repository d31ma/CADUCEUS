package syncing

import (
	"time"

	"mesail.dev/packages/core"
)

type syncWindow struct {
	now func() time.Time
}

func (window syncWindow) usable(after string) (core.ChangeCursor, bool) {
	cursor, valid := core.ChangeCursors{}.Parse(after)
	return cursor, valid && !cursor.At.Before(window.keptFrom())
}

func (window syncWindow) keptFrom() time.Time {
	return core.ChangeRetention{}.KeptFrom(window.now())
}

func (window syncWindow) overlapStart(after core.ChangeCursor) time.Time {
	return after.At.Add(-window.overlap())
}

func (window syncWindow) months(from time.Time) []string {
	ids := core.MessageIds{}
	first := window.monthStart(window.later(from.UTC(), window.keptFrom()))
	months := []string{}
	for month := first; !month.After(window.now().UTC()); month = month.AddDate(0, 1, 0) {
		months = append(months, ids.Month(month))
	}
	return months
}

func (syncWindow) monthStart(at time.Time) time.Time {
	return time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func (syncWindow) overlap() time.Duration {
	return 5 * time.Second
}

func (syncWindow) pageSize() int {
	return 200
}

func (syncWindow) later(first time.Time, second time.Time) time.Time {
	if first.After(second) {
		return first
	}
	return second
}
