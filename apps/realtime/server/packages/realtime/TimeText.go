package realtime

import "time"

type TimeText struct{}

func (TimeText) At(text string) time.Time {
	at, _ := time.Parse(time.RFC3339, text)
	return at
}

func (TimeText) Format(at time.Time) string {
	return at.UTC().Format(time.RFC3339)
}
