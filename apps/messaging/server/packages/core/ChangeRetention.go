package core

import "time"

type ChangeRetention struct{}

func (retention ChangeRetention) KeptFrom(now time.Time) time.Time {
	at := now.UTC()
	return time.Date(at.Year(), at.Month()-time.Month(retention.keptMonths()-1), 1, 0, 0, 0, 0, time.UTC)
}

func (retention ChangeRetention) FirstKeptMonth(now time.Time) string {
	return MessageIds{}.Month(retention.KeptFrom(now))
}

func (ChangeRetention) keptMonths() int {
	return 3
}
