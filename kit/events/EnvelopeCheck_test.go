package events

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func validEnvelope() Envelope {
	return Envelope{
		EventID:    "evt_00000000000000000000000000000001",
		Type:       Types{}.MessageCreated(),
		DomainId:   "example.com",
		Producer:   Services{}.Messaging(),
		OccurredAt: "2026-11-10T10:30:00.5Z",
		Payload:    json.RawMessage(`{"messageId":"msg_1"}`),
	}
}

func checkEnvelopeRefused(t *testing.T, envelope Envelope) {
	t.Helper()
	var invalid InvalidEnvelope
	err := EnvelopeCheck{}.Check(envelope)
	if !errors.As(err, &invalid) {
		t.Fatalf("expected InvalidEnvelope, got %v", err)
	}
}

func TestEnvelopeCheckAcceptsValidEnvelope(t *testing.T) {
	err := EnvelopeCheck{}.Check(validEnvelope())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestEnvelopeCheckRefusesEmptyEventId(t *testing.T) {
	envelope := validEnvelope()
	envelope.EventID = ""
	checkEnvelopeRefused(t, envelope)
}

func TestEnvelopeCheckRefusesBadDomain(t *testing.T) {
	envelope := validEnvelope()
	envelope.DomainId = "example"
	checkEnvelopeRefused(t, envelope)
}

func TestEnvelopeCheckRefusesUnknownType(t *testing.T) {
	envelope := validEnvelope()
	envelope.Type = "message.exploded"
	checkEnvelopeRefused(t, envelope)
}

func TestEnvelopeCheckRefusesWrongProducer(t *testing.T) {
	envelope := validEnvelope()
	envelope.Producer = Services{}.Channels()
	checkEnvelopeRefused(t, envelope)
}

func TestEnvelopeCheckRefusesBadTime(t *testing.T) {
	envelope := validEnvelope()
	envelope.OccurredAt = "yesterday"
	checkEnvelopeRefused(t, envelope)
}

func TestEnvelopeCheckRefusesEmptyPayload(t *testing.T) {
	envelope := validEnvelope()
	envelope.Payload = nil
	checkEnvelopeRefused(t, envelope)
}

func TestEnvelopeCheckAcceptsTheOtherListedProducer(t *testing.T) {
	factory := NewEnvelopeFactory(Services{}.Notify(), time.Now, rand.Reader)
	_, err := factory.New(Types{}.EmailSendRequested(), "example.com", EmailSendRequested{})
	if err != nil {
		t.Fatalf("got %v", err)
	}
}
