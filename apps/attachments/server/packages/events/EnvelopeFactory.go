package events

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"time"
)

type EnvelopeFactory struct {
	producer string
	now      func() time.Time
	random   io.Reader
}

func NewEnvelopeFactory(producer string, now func() time.Time, random io.Reader) EnvelopeFactory {
	return EnvelopeFactory{producer: producer, now: now, random: random}
}

func (factory EnvelopeFactory) New(eventType string, domainId string, payload any) (Envelope, error) {
	err := EnvelopeCheck{}.Producer(eventType, factory.producer)
	if err != nil {
		return Envelope{}, err
	}
	err = DomainName{}.Check(domainId)
	if err != nil {
		return Envelope{}, err
	}
	eventId, err := factory.newEventId()
	if err != nil {
		return Envelope{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		EventID:    eventId,
		Type:       eventType,
		DomainId:   domainId,
		Producer:   factory.producer,
		OccurredAt: factory.now().UTC().Format(time.RFC3339Nano),
		Payload:    body,
	}, nil
}

func (EnvelopeFactory) eventIdPrefix() string {
	return "evt_"
}

func (EnvelopeFactory) randomByteCount() int {
	return 16
}

func (factory EnvelopeFactory) newEventId() (string, error) {
	randomBytes := make([]byte, factory.randomByteCount())
	_, err := io.ReadFull(factory.random, randomBytes)
	return factory.eventIdPrefix() + hex.EncodeToString(randomBytes), err
}
