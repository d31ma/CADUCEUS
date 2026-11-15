package devicebox

import (
	"crypto/rand"
	"errors"
	"slices"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

type EventStaging struct {
	settings Settings
	now      func() time.Time
}

func NewEventStaging(settings Settings, now func() time.Time) EventStaging {
	return EventStaging{settings: settings, now: now}
}

func (staging EventStaging) Staged(domainId string, delivery Delivery) (outbox.Fields, error) {
	err := staging.settings.Checked()
	if err != nil {
		return outbox.Fields{}, err
	}
	fields := outbox.Fields{PendingEvents: []string{}, DeadEvents: []string{}}
	for _, personId := range staging.persons(delivery) {
		fields, err = staging.stageOne(fields, domainId, staging.payload(delivery, personId))
		if err != nil {
			return outbox.Fields{}, staging.outboxError(err)
		}
	}
	return fields, nil
}

func (staging EventStaging) stageOne(fields outbox.Fields, domainId string, payload events.EnvelopeStored) (outbox.Fields, error) {
	factory := events.NewEnvelopeFactory(events.Services{}.Devicebox(), staging.now, rand.Reader)
	envelope, err := factory.New(events.Types{}.EnvelopeStored(), domainId, payload)
	if err != nil {
		return fields, err
	}
	return outbox.NewOutbox(staging.settings.Outbox).Stage(fields, envelope)
}

func (EventStaging) persons(delivery Delivery) []string {
	persons := []string{}
	for _, deviceId := range delivery.DeviceIds {
		persons = append(persons, delivery.Owners[deviceId])
	}
	slices.Sort(persons)
	return slices.Compact(persons)
}

func (EventStaging) payload(delivery Delivery, personId string) events.EnvelopeStored {
	deviceIds := []string{}
	for _, deviceId := range delivery.DeviceIds {
		deviceIds = append(deviceIds, EventStaging{}.ifOwnedBy(delivery.Owners[deviceId], personId, deviceId)...)
	}
	return events.EnvelopeStored{ToPersonId: personId, ToDeviceIds: deviceIds, Kind: delivery.Kind, EnvelopeId: delivery.EnvelopeId}
}

func (EventStaging) ifOwnedBy(ownerId string, personId string, deviceId string) []string {
	if ownerId != personId {
		return nil
	}
	return []string{deviceId}
}

func (EventStaging) outboxError(failure error) error {
	full := outbox.OutboxFull{}
	if errors.As(failure, &full) {
		return Unavailable{}.OutboxFull(full)
	}
	return Unavailable{}.OutboxUnreadable(failure)
}
