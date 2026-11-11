package channels

import "strconv"

type SweepRefused struct {
	Status int
}

func (failure SweepRefused) Error() string {
	return "the outbox relay route answered " + strconv.Itoa(failure.Status)
}
