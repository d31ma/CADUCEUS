package fakes

import "time"

type Clock struct {
	At time.Time
}

func (clock *Clock) Now() time.Time {
	return clock.At
}

func (clock *Clock) Advance(step time.Duration) {
	clock.At = clock.At.Add(step)
}
