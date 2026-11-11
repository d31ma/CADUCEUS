package mailout

import "time"

type Clock struct{}

func (Clock) Format(moment time.Time) string {
	return moment.UTC().Format(time.RFC3339)
}

func (clock Clock) Parse(text string) time.Time {
	return clock.momentOrZero(time.Parse(time.RFC3339, text))
}

func (Clock) momentOrZero(moment time.Time, err error) time.Time {
	if err != nil {
		return time.Time{}
	}
	return moment
}
