package events

import (
	"errors"
	"testing"

	"mesail.dev/packages/oidc"
)

type recordingHandler struct {
	handled []Envelope
	failure error
}

func (handler *recordingHandler) Handle(envelope Envelope) error {
	handler.handled = append(handler.handled, envelope)
	return handler.failure
}

type brokenSeen struct {
	seenFailure     error
	rememberFailure error
}

func (seen brokenSeen) Seen(string, string) (bool, error) {
	return false, seen.seenFailure
}

func (seen brokenSeen) Remember(string, string) error {
	return seen.rememberFailure
}

func serviceCaller() oidc.Caller {
	return oidc.Caller{Roles: []string{"Member", "Service"}}
}

func searchReceiver(seen SeenStore, handler Handler) Receiver {
	return NewReceiver(Services{}.Search(), seen, handler)
}

func TestReceiverAppliesValidEnvelopeOnce(t *testing.T) {
	handler := &recordingHandler{}
	seen := NewMemorySeen()
	receipt, err := searchReceiver(seen, handler).Receive(serviceCaller(), validEnvelope())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if receipt.Status != "applied" || receipt.EventID != validEnvelope().EventID {
		t.Fatalf("unexpected receipt %+v", receipt)
	}
	if len(handler.handled) != 1 || seen.Count() != 1 {
		t.Fatalf("expected one handled and remembered event, got %d and %d", len(handler.handled), seen.Count())
	}
}

func TestReceiverReportsDuplicateWithoutHandling(t *testing.T) {
	handler := &recordingHandler{}
	receiver := searchReceiver(NewMemorySeen(), handler)
	_, _ = receiver.Receive(serviceCaller(), validEnvelope())
	receipt, err := receiver.Receive(serviceCaller(), validEnvelope())
	if err != nil || receipt.Status != "duplicate" || receipt.EventID != validEnvelope().EventID {
		t.Fatalf("unexpected result %+v, %v", receipt, err)
	}
	if len(handler.handled) != 1 {
		t.Fatalf("expected the handler to run once, ran %d times", len(handler.handled))
	}
}

func TestReceiverAppliesSameEventIdInAnotherDomain(t *testing.T) {
	handler := &recordingHandler{}
	receiver := searchReceiver(NewMemorySeen(), handler)
	other := validEnvelope()
	other.DomainId = "example.org"
	_, _ = receiver.Receive(serviceCaller(), validEnvelope())
	receipt, err := receiver.Receive(serviceCaller(), other)
	if err != nil || receipt.Status != "applied" || len(handler.handled) != 2 {
		t.Fatalf("unexpected result %+v, %v, handled %d", receipt, err, len(handler.handled))
	}
}

func TestReceiverRefusesCallerWithoutServiceRole(t *testing.T) {
	handler := &recordingHandler{}
	var notService NotService
	_, err := searchReceiver(NewMemorySeen(), handler).Receive(oidc.Caller{Roles: []string{"DomainAdmin"}}, validEnvelope())
	if !errors.As(err, &notService) || len(handler.handled) != 0 {
		t.Fatalf("expected NotService, got %v", err)
	}
}

func TestReceiverRefusesInvalidEnvelope(t *testing.T) {
	handler := &recordingHandler{}
	envelope := validEnvelope()
	envelope.Producer = Services{}.Channels()
	var invalid InvalidEnvelope
	_, err := searchReceiver(NewMemorySeen(), handler).Receive(serviceCaller(), envelope)
	if !errors.As(err, &invalid) || len(handler.handled) != 0 {
		t.Fatalf("expected InvalidEnvelope, got %v", err)
	}
}

func TestReceiverRefusesUnsubscribedService(t *testing.T) {
	handler := &recordingHandler{}
	var notSubscribed NotSubscribed
	_, err := NewReceiver(Services{}.MailOut(), NewMemorySeen(), handler).Receive(serviceCaller(), validEnvelope())
	if !errors.As(err, &notSubscribed) || notSubscribed.Service != "mail-out" || notSubscribed.Type != "message.created" {
		t.Fatalf("expected NotSubscribed, got %v", err)
	}
}

func TestReceiverDoesNotRememberWhenHandlerFails(t *testing.T) {
	handler := &recordingHandler{failure: errors.New("store is down")}
	seen := NewMemorySeen()
	receiver := searchReceiver(seen, handler)
	_, err := receiver.Receive(serviceCaller(), validEnvelope())
	if err == nil || seen.Count() != 0 {
		t.Fatalf("expected the handler error and nothing remembered, got %v and %d", err, seen.Count())
	}
	handler.failure = nil
	receipt, err := receiver.Receive(serviceCaller(), validEnvelope())
	if err != nil || receipt.Status != "applied" {
		t.Fatalf("expected the retry to apply, got %+v, %v", receipt, err)
	}
}

func TestReceiverReportsSeenLookupError(t *testing.T) {
	handler := &recordingHandler{}
	failure := errors.New("seen lookup failed")
	_, err := searchReceiver(brokenSeen{seenFailure: failure}, handler).Receive(serviceCaller(), validEnvelope())
	if !errors.Is(err, failure) || len(handler.handled) != 0 {
		t.Fatalf("expected the lookup error, got %v", err)
	}
}

func TestReceiverReportsRememberError(t *testing.T) {
	handler := &recordingHandler{}
	failure := errors.New("remember failed")
	_, err := searchReceiver(brokenSeen{rememberFailure: failure}, handler).Receive(serviceCaller(), validEnvelope())
	if !errors.Is(err, failure) {
		t.Fatalf("expected the remember error, got %v", err)
	}
}
