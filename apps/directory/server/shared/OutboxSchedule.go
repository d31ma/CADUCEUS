import (
	"time"

	"mesail.dev/packages/outbox"
)

type OutboxSchedule struct{}

func (schedule OutboxSchedule) Due(document outbox.Document, now time.Time) bool {
	state, failure := outbox.Codec{}.Read(document.Fields)
	return failure == nil && schedule.anyDue(state.Pending, now)
}

func (OutboxSchedule) anyDue(pending []outbox.PendingEvent, now time.Time) bool {
	for _, event := range pending {
		next, _ := time.Parse(time.RFC3339Nano, event.NextAttemptAt)
		if !next.After(now) {
			return true
		}
	}
	return false
}
