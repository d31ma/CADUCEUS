package outbox

import "mesail.dev/packages/events"

type PendingEvent struct {
	Envelope      events.Envelope `json:"envelope"`
	Waiting       []string        `json:"waiting"`
	Attempts      int             `json:"attempts"`
	NextAttemptAt string          `json:"nextAttemptAt"`
	LastError     string          `json:"lastError"`
}
