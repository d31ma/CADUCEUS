package events

import (
	"bytes"
	"errors"
	"regexp"
	"testing"
	"time"
)

type failingRandom struct{}

func (failingRandom) Read([]byte) (int, error) {
	return 0, errors.New("random source failed")
}

func fixedNow() time.Time {
	return time.Date(2026, 11, 10, 12, 30, 0, 123456789, time.FixedZone("plus two", 2*60*60))
}

func messagingFactory() EnvelopeFactory {
	return NewEnvelopeFactory(Services{}.Messaging(), fixedNow, bytes.NewReader(bytes.Repeat([]byte{0xab}, 32)))
}

func TestEnvelopeFactoryBuildsEnvelope(t *testing.T) {
	envelope, err := messagingFactory().New(Types{}.MessageCreated(), "example.com", map[string]string{"messageId": "msg_1"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if envelope.EventID != "evt_abababababababababababababababab" {
		t.Fatalf("unexpected event id %q", envelope.EventID)
	}
	if envelope.OccurredAt != "2026-11-10T10:30:00.123456789Z" {
		t.Fatalf("unexpected time %q", envelope.OccurredAt)
	}
	if string(envelope.Payload) != `{"messageId":"msg_1"}` {
		t.Fatalf("unexpected payload %s", envelope.Payload)
	}
	if envelope.Producer != "messaging" || envelope.Type != "message.created" || envelope.DomainId != "example.com" {
		t.Fatalf("unexpected envelope %+v", envelope)
	}
}

func TestEnvelopeFactoryEventIdShape(t *testing.T) {
	factory := NewEnvelopeFactory(Services{}.Messaging(), fixedNow, bytes.NewReader([]byte("0123456789abcdef")))
	envelope, err := factory.New(Types{}.MessageCreated(), "example.com", struct{}{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !regexp.MustCompile(`^evt_[0-9a-f]{32}$`).MatchString(envelope.EventID) {
		t.Fatalf("unexpected event id %q", envelope.EventID)
	}
}

func TestEnvelopeFactoryRefusesWrongProducer(t *testing.T) {
	factory := NewEnvelopeFactory(Services{}.Channels(), fixedNow, bytes.NewReader(make([]byte, 16)))
	var invalid InvalidEnvelope
	_, err := factory.New(Types{}.MessageCreated(), "example.com", struct{}{})
	if !errors.As(err, &invalid) {
		t.Fatalf("expected InvalidEnvelope, got %v", err)
	}
}

func TestEnvelopeFactoryRefusesUnknownType(t *testing.T) {
	var invalid InvalidEnvelope
	_, err := messagingFactory().New("message.exploded", "example.com", struct{}{})
	if !errors.As(err, &invalid) {
		t.Fatalf("expected InvalidEnvelope, got %v", err)
	}
}

func TestEnvelopeFactoryRefusesBadDomain(t *testing.T) {
	var invalid InvalidEnvelope
	_, err := messagingFactory().New(Types{}.MessageCreated(), "Example.com", struct{}{})
	if !errors.As(err, &invalid) {
		t.Fatalf("expected InvalidEnvelope, got %v", err)
	}
}

func TestEnvelopeFactoryReportsRandomSourceError(t *testing.T) {
	factory := NewEnvelopeFactory(Services{}.Messaging(), fixedNow, failingRandom{})
	_, err := factory.New(Types{}.MessageCreated(), "example.com", struct{}{})
	if err == nil || err.Error() != "random source failed" {
		t.Fatalf("expected the random source error, got %v", err)
	}
}

func TestEnvelopeFactoryReportsShortRandomSource(t *testing.T) {
	factory := NewEnvelopeFactory(Services{}.Messaging(), fixedNow, bytes.NewReader(make([]byte, 4)))
	_, err := factory.New(Types{}.MessageCreated(), "example.com", struct{}{})
	if err == nil {
		t.Fatalf("expected an error for a short random source")
	}
}

func TestEnvelopeFactoryReportsPayloadEncodingError(t *testing.T) {
	_, err := messagingFactory().New(Types{}.MessageCreated(), "example.com", make(chan int))
	if err == nil {
		t.Fatalf("expected an encoding error")
	}
}
