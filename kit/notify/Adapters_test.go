package notify

import (
	"testing"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

func TestEventHandlerFunctionCallsTheFunction(t *testing.T) {
	handled := ""
	var handler events.Handler = EventHandlerFunction(func(envelope events.Envelope) error {
		handled = envelope.EventID
		return nil
	})
	err := handler.Handle(events.Envelope{EventID: "evt_1"})
	if err != nil || handled != "evt_1" {
		t.Fatalf("handled %q, error %v", handled, err)
	}
}

func TestSeenStoreFunctionsCallTheFunctions(t *testing.T) {
	remembered := ""
	var store events.SeenStore = SeenStoreFunctions{
		SeenFunction:     func(domainId string, eventId string) (bool, error) { return eventId == "evt_1", nil },
		RememberFunction: func(domainId string, eventId string) error { remembered = domainId + "/" + eventId; return nil },
	}
	seen, err := store.Seen("example.org", "evt_1")
	remembering := store.Remember("example.org", "evt_2")
	if !seen || err != nil || remembering != nil || remembered != "example.org/evt_2" {
		t.Fatalf("seen %v, remembered %q", seen, remembered)
	}
}

func TestOutboxStoreFunctionsCallTheFunctions(t *testing.T) {
	memory := outbox.NewMemoryStore()
	ref := outbox.Ref{Collection: "notify-summaries", Id: "1"}
	var store outbox.Store = OutboxStoreFunctions{PendingFunction: memory.Pending, LoadFunction: memory.Load, SaveFunction: memory.Save}
	saving := store.Save(ref, outbox.Fields{PendingEvents: []string{}, DeadEvents: []string{}, OutboxPending: 1, OutboxWaiting: true})
	fields, loading := store.Load(ref)
	pending, listing := store.Pending(10)
	if saving != nil || loading != nil || listing != nil || fields.OutboxPending != 1 || len(pending) != 1 {
		t.Fatalf("fields %+v, pending %d, errors %v %v %v", fields, len(pending), saving, loading, listing)
	}
}
