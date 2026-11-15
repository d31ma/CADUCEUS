package events

import (
	"slices"
	"strings"
	"time"
)

type EnvelopeCheck struct{}

func (check EnvelopeCheck) Check(envelope Envelope) error {
	err := check.requireEventId(envelope.EventID)
	if err != nil {
		return err
	}
	err = DomainName{}.Check(envelope.DomainId)
	if err != nil {
		return err
	}
	err = check.Producer(envelope.Type, envelope.Producer)
	if err != nil {
		return err
	}
	err = check.requireTime(envelope.OccurredAt)
	if err != nil {
		return err
	}
	return check.requirePayload(envelope)
}

func (check EnvelopeCheck) Producer(eventType string, producer string) error {
	expected, err := check.knownProducers(eventType)
	if err != nil {
		return err
	}
	return check.listedProducer(expected, producer)
}

func (EnvelopeCheck) requireEventId(eventId string) error {
	if eventId == "" {
		return InvalidEnvelope{}.EmptyEventId()
	}
	return nil
}

func (EnvelopeCheck) requireTime(occurredAt string) error {
	_, err := time.Parse(time.RFC3339, occurredAt)
	if err != nil {
		return InvalidEnvelope{}.OccurredAtNotATime()
	}
	return nil
}

func (EnvelopeCheck) requirePayload(envelope Envelope) error {
	if len(envelope.Payload) == 0 {
		return InvalidEnvelope{}.EmptyPayload()
	}
	return nil
}

func (EnvelopeCheck) knownProducers(eventType string) ([]string, error) {
	producers, err := Catalog{}.ProducersOf(eventType)
	if err != nil {
		return nil, InvalidEnvelope{}.UnknownType(UnknownEventType{Type: eventType})
	}
	return producers, nil
}

func (EnvelopeCheck) listedProducer(expected []string, producer string) error {
	if !slices.Contains(expected, producer) {
		return InvalidEnvelope{}.WrongProducer(producer, strings.Join(expected, " or "))
	}
	return nil
}
