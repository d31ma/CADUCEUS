package outbox

import (
	"slices"
	"time"
)

type dueDocument struct {
	document Document
	times    []time.Time
	readable bool
}

func (candidate dueDocument) due(now time.Time) bool {
	return !candidate.readable || candidate.dueBy(now)
}

func (candidate dueDocument) dueBy(now time.Time) bool {
	return slices.ContainsFunc(candidate.times, func(moment time.Time) bool { return !moment.After(now) })
}

func (candidate dueDocument) earliest() time.Time {
	if len(candidate.times) == 0 {
		return time.Time{}
	}
	return slices.MinFunc(candidate.times, time.Time.Compare)
}

func (candidate dueDocument) rank() int {
	if candidate.readable {
		return 0
	}
	return 1
}
