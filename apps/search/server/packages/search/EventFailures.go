package search

import (
	"errors"

	"mesail.dev/packages/events"
)

type EventFailures struct{}

func (failures EventFailures) Translated(failure error) error {
	switch {
	case errors.As(failure, &events.InvalidEnvelope{}), errors.As(failure, &events.NotSubscribed{}):
		return InvalidRequest{Message: failure.Error(), Field: failures.envelope()}
	case errors.As(failure, &events.NotService{}):
		return Forbidden{Message: failure.Error()}
	default:
		return failure
	}
}

func (EventFailures) envelope() string {
	return "envelope"
}
