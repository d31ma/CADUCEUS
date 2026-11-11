package outbox

import (
	"encoding/json"
	"testing"
	"time"

	"mesail.dev/packages/events"
)

func testSettings() Settings {
	return Settings{
		Producer: "channels",
		Subscribers: []Subscriber{
			{Name: "messaging", URL: "http://messaging.test/events"},
			{Name: "realtime", URL: "http://realtime.test/events"},
		},
	}
}

func testEnvelope(eventId string, eventType string) events.Envelope {
	return events.Envelope{
		EventID:    eventId,
		Type:       eventType,
		DomainId:   "example.com",
		Producer:   "channels",
		OccurredAt: "2026-11-10T12:00:00Z",
		Payload:    json.RawMessage(`{"channelId":"chn_1"}`),
	}
}

func channelCreated(eventId string) events.Envelope {
	return testEnvelope(eventId, events.Types{}.ChannelCreated())
}

func startTime() time.Time {
	return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
}

func stageAll(t *testing.T, settings Settings, envelopes ...events.Envelope) Fields {
	t.Helper()
	fields := Fields{}
	for _, envelope := range envelopes {
		staged, err := NewOutbox(settings).Stage(fields, envelope)
		if err != nil {
			t.Fatal(err)
		}
		fields = staged
	}
	return fields
}

func pendingOf(t *testing.T, fields Fields) []PendingEvent {
	t.Helper()
	state, err := Codec{}.Read(fields)
	if err != nil {
		t.Fatal(err)
	}
	return state.Pending
}

func TestStageAddsAPendingEventForEveryConfiguredConsumer(t *testing.T) {
	fields := stageAll(t, testSettings(), channelCreated("evt_1"))

	pending := pendingOf(t, fields)
	if fields.OutboxPending != 1 || len(pending) != 1 {
		t.Fatalf("fields %+v", fields)
	}
	if got := pending[0].Waiting; len(got) != 2 || got[0] != "messaging" || got[1] != "realtime" {
		t.Fatalf("waiting %v", got)
	}
	if pending[0].Attempts != 0 || pending[0].NextAttemptAt != "" {
		t.Fatalf("event %+v", pending[0])
	}
}

func TestStageKeepsEarlierEventsInOrder(t *testing.T) {
	fields := stageAll(t, testSettings(), channelCreated("evt_1"), channelCreated("evt_2"))

	pending := pendingOf(t, fields)
	if fields.OutboxPending != 2 || pending[0].Envelope.EventID != "evt_1" || pending[1].Envelope.EventID != "evt_2" {
		t.Fatalf("pending %+v", pending)
	}
}

func TestStageLeavesTheFieldsAloneWhenNoConsumerIsConfigured(t *testing.T) {
	fields := stageAll(t, testSettings(), testEnvelope("evt_1", events.Types{}.EmailSendRequested()))

	if fields.OutboxPending != 0 || len(fields.PendingEvents) != 0 {
		t.Fatalf("fields %+v", fields)
	}
}

func TestStageRefusesFieldsItCannotRead(t *testing.T) {
	broken := Fields{PendingEvents: []string{"not json"}, OutboxPending: 1}

	fields, err := NewOutbox(testSettings()).Stage(broken, channelCreated("evt_1"))
	if err == nil || fields.PendingEvents[0] != "not json" {
		t.Fatalf("fields %+v, error %v", fields, err)
	}
}
