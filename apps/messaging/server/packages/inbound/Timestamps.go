package inbound

import "time"

type Timestamps struct{}

func (Timestamps) Of(at time.Time) string {
	return at.UTC().Format(time.RFC3339)
}
