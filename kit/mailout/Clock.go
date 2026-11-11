package mailout

import "time"

type Clock struct{}

func (Clock) Format(moment time.Time) string {
	return moment.UTC().Format(time.RFC3339)
}

func (Clock) Parse(text string) time.Time {
	moment, _ := time.Parse(time.RFC3339, text)
	return moment
}
