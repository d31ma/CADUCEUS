package sending

import (
	"time"

	"mesail.dev/packages/core"
)

type Months struct{}

func (Months) recent(now time.Time) []string {
	at := now.UTC()
	first := time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
	return []string{core.MessageIds{}.Month(at), core.MessageIds{}.Month(first.AddDate(0, -1, 0))}
}

func (Months) stamp(now time.Time) string {
	return now.UTC().Format(time.RFC3339)
}
