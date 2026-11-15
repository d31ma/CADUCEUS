package events

import "testing"

func TestInvalidEnvelopeMessage(t *testing.T) {
	message := InvalidEnvelope{Reason: "payload must not be empty"}.Error()
	if message != "the event envelope is not valid: payload must not be empty" {
		t.Fatalf("unexpected message %q", message)
	}
}

func TestInvalidEnvelopeCarriesTheNumberOfItsCase(t *testing.T) {
	if (InvalidEnvelope{}).PayloadShape().Number() != 11 {
		t.Fatalf("got %d", InvalidEnvelope{}.PayloadShape().Number())
	}
}

func TestInvalidEnvelopeForAnUnknownTypeCarriesTheCatalogNumber(t *testing.T) {
	refused := InvalidEnvelope{}.UnknownType(UnknownEventType{Type: "x"})
	if refused.Number() != 6 || refused.Reason != (UnknownEventType{Type: "x"}).Error() {
		t.Fatalf("got %+v", refused)
	}
}
