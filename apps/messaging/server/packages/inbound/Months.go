package inbound

import (
	"time"

	"mesail.dev/packages/core"
)

type Months struct{}

func (Months) CurrentAndPrevious(now time.Time) []string {
	utc := now.UTC()
	firstDay := time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	ids := core.MessageIds{}
	return []string{ids.Month(utc), ids.Month(firstDay.Add(-time.Hour))}
}
