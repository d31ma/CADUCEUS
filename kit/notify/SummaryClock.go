package notify

import "time"

type SummaryClock struct{}

func (clock SummaryClock) DueAt(createdAt time.Time, afterMinutes int) int64 {
	return clock.dueAt(createdAt, afterMinutes)
}

func (SummaryClock) dueAt(createdAt time.Time, afterMinutes int) int64 {
	if afterMinutes == 0 {
		return 0
	}
	return createdAt.Add(time.Duration(afterMinutes) * time.Minute).Unix()
}
