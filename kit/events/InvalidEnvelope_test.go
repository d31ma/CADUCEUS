package events

import "testing"

func TestInvalidEnvelopeMessage(t *testing.T) {
	message := InvalidEnvelope{Reason: "payload must not be empty"}.Error()
	if message != "the event envelope is not valid: payload must not be empty" {
		t.Fatalf("unexpected message %q", message)
	}
}
