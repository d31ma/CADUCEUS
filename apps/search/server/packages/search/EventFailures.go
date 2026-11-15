package search

import (
	"cmp"
	"errors"

	"mesail.dev/packages/events"
)

type EventFailures struct{}

func (failures EventFailures) Translated(failure error) error {
	return cmp.Or(failures.envelope(failure), failures.unsubscribed(failure), failures.notService(failure), failure)
}

func (EventFailures) field() string {
	return "envelope"
}

func (failures EventFailures) envelope(failure error) error {
	invalid := events.InvalidEnvelope{}
	if !errors.As(failure, &invalid) {
		return nil
	}
	return InvalidRequest{}.Kit(invalid.Error(), failures.field(), invalid.Number())
}

func (failures EventFailures) unsubscribed(failure error) error {
	unsubscribed := events.NotSubscribed{}
	if !errors.As(failure, &unsubscribed) {
		return nil
	}
	return InvalidRequest{}.Kit(unsubscribed.Error(), failures.field(), unsubscribed.Number())
}

func (EventFailures) notService(failure error) error {
	notService := events.NotService{}
	if !errors.As(failure, &notService) {
		return nil
	}
	return Forbidden{}.Kit(notService.Error(), notService.Number())
}
