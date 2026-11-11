package core

import (
	"io"
	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
	"time"
)

type Stager struct {
	settings outbox.Settings
	now      func() time.Time
	random   io.Reader
}

func NewStager(settings outbox.Settings, now func() time.Time, random io.Reader) Stager {
	return Stager{settings: settings, now: now, random: random}
}

func (stager Stager) Stage(fields OutboxFields, eventType string, domainId string, payload any) (OutboxFields, error) {
	envelope, err := events.NewEnvelopeFactory(stager.producer(), stager.now, stager.random).New(eventType, domainId, payload)
	if err != nil {
		return fields, err
	}
	staged, err := outbox.NewOutbox(stager.settings).Stage(fields.Fields(), envelope)
	return OutboxFields{}.From(staged), err
}

func (Stager) producer() string {
	return "messaging"
}
