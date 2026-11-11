package mailout

import "time"

type Backoff struct {
	First   time.Duration
	Largest time.Duration
}

func (backoff Backoff) Delay(attempts int) time.Duration {
	shift := min(max(attempts-1, 0), 20)
	return min(backoff.First*time.Duration(1<<shift), backoff.Largest)
}
