package events

import "time"

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
	expected, err := check.knownProducer(eventType)
	if err != nil {
		return err
	}
	return check.sameProducer(expected, producer)
}

func (EnvelopeCheck) requireEventId(eventId string) error {
	if eventId == "" {
		return InvalidEnvelope{Reason: "eventId must not be empty"}
	}
	return nil
}

func (EnvelopeCheck) requireTime(occurredAt string) error {
	_, err := time.Parse(time.RFC3339, occurredAt)
	if err != nil {
		return InvalidEnvelope{Reason: "occurredAt must be an RFC 3339 time"}
	}
	return nil
}

func (EnvelopeCheck) requirePayload(envelope Envelope) error {
	if len(envelope.Payload) == 0 {
		return InvalidEnvelope{Reason: "payload must not be empty"}
	}
	return nil
}

func (EnvelopeCheck) knownProducer(eventType string) (string, error) {
	producer, err := Catalog{}.ProducerOf(eventType)
	if err != nil {
		return "", InvalidEnvelope{Reason: err.Error()}
	}
	return producer, nil
}

func (EnvelopeCheck) sameProducer(expected string, producer string) error {
	if expected != producer {
		return InvalidEnvelope{Reason: "the producer " + producer + " does not produce this event type; " + expected + " does"}
	}
	return nil
}
