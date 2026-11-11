package reading

import "time"

type clock struct {
	now func() time.Time
}

func (clock clock) stamp() string {
	return clock.now().UTC().Format(time.RFC3339)
}
