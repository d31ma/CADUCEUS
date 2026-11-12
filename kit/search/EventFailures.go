package search

import (
	"errors"

	"mesail.dev/packages/events"
)

type EventFailures struct{}

func (failures EventFailures) Translated(failure error) error {
	switch {
	case errors.As(failure, &events.InvalidEnvelope{}), errors.As(failure, &events.NotSubscribed{}):
		return InvalidRequest{}.BadEnvelope(failure.Error())
	case errors.As(failure, &events.NotService{}):
		return Forbidden{}.ServicesOnly(failure.Error())
	default:
		return failure
	}
}
