package events

import (
	"encoding/json"
	"errors"
	"testing"
)

type messagePayload struct {
	MessageId string `json:"messageId"`
}

func TestPayloadReaderReadsKnownFields(t *testing.T) {
	var payload messagePayload
	err := PayloadReader{}.Read(validEnvelope(), &payload)
	if err != nil || payload.MessageId != "msg_1" {
		t.Fatalf("unexpected result %+v, %v", payload, err)
	}
}

func TestPayloadReaderRefusesUnknownFields(t *testing.T) {
	envelope := validEnvelope()
	envelope.Payload = json.RawMessage(`{"messageId":"msg_1","extra":true}`)
	var payload messagePayload
	err := PayloadReader{}.Read(envelope, &payload)
	if err == nil {
		t.Fatalf("expected an unknown field error")
	}
}

func TestPayloadReaderRefusesTrailingData(t *testing.T) {
	envelope := validEnvelope()
	envelope.Payload = json.RawMessage(`{"messageId":"msg_1"}{"messageId":"msg_2"}`)
	var payload messagePayload
	var invalid InvalidEnvelope
	err := PayloadReader{}.Read(envelope, &payload)
	if !errors.As(err, &invalid) {
		t.Fatalf("expected InvalidEnvelope, got %v", err)
	}
}
