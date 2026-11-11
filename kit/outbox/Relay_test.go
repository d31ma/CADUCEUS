package outbox

import (
	"errors"
	"testing"
	"time"

	"mesail.dev/packages/events"
)

type relayFixture struct {
	store  *MemoryStore
	pusher *RecordingPusher
	now    time.Time
	relay  Relay
	ref    Ref
}

func newRelayFixture(t *testing.T, settings Settings, envelopes ...events.Envelope) *relayFixture {
	t.Helper()
	fixture := &relayFixture{store: NewMemoryStore(), pusher: NewRecordingPusher(), now: startTime(), ref: Ref{Collection: "channels", Id: "chn_1"}}
	fixture.store.Put(fixture.ref, stageAll(t, settings, envelopes...))
	fixture.relay = NewRelay(fixture.store, fixture.pusher, settings, func() time.Time { return fixture.now })
	return fixture
}

func (fixture *relayFixture) run(t *testing.T) Report {
	t.Helper()
	report, err := fixture.relay.RunOnce(10)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func eventIds(envelopes []events.Envelope) []string {
	ids := []string{}
	for _, envelope := range envelopes {
		ids = append(ids, envelope.EventID)
	}
	return ids
}

type hookPusher struct {
	inner *RecordingPusher
	hook  func()
}

func (pusher hookPusher) Push(subscriber Subscriber, envelope events.Envelope) error {
	pusher.hook()
	return pusher.inner.Push(subscriber, envelope)
}

func TestRelayDeliversToEveryConfiguredConsumerOnce(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))

	report := fixture.run(t)
	again := fixture.run(t)

	if report != (Report{Documents: 1, Delivered: 2}) || again != (Report{}) {
		t.Fatalf("reports %+v and %+v", report, again)
	}
	if len(fixture.pusher.PushedTo("messaging")) != 1 || len(fixture.pusher.PushedTo("realtime")) != 1 {
		t.Fatalf("pushed %+v", fixture.pusher.Pushed())
	}
	if fields := fixture.store.Fields(fixture.ref); fields.OutboxPending != 0 || len(fields.PendingEvents) != 0 {
		t.Fatalf("fields %+v", fields)
	}
}

func TestRelaySkipsAConsumerNotListedInSettings(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))

	fixture.run(t)

	if len(fixture.pusher.PushedTo("search")) != 0 || len(fixture.pusher.Attempted()) != 2 {
		t.Fatalf("attempted %+v", fixture.pusher.Attempted())
	}
}

func TestRelayDocumentDrainsOneDocument(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))

	report, err := fixture.relay.RelayDocument(fixture.ref)

	if err != nil || report != (Report{Documents: 1, Delivered: 2}) {
		t.Fatalf("report %+v, error %v", report, err)
	}
}

func TestRelayRetriesATransientFailureAfterTheBackoffAndKeepsOrder(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"), channelCreated("evt_2"))
	fixture.pusher.FailFor("messaging", errors.New("messaging is down"))

	first := fixture.run(t)
	if first != (Report{Documents: 1, Delivered: 2, Retried: 1}) {
		t.Fatalf("first report %+v", first)
	}
	pending := pendingOf(t, fixture.store.Fields(fixture.ref))
	if len(pending) != 2 || pending[0].Attempts != 1 || pending[0].NextAttemptAt != "2026-11-10T12:00:05Z" || pending[0].LastError != "messaging is down" {
		t.Fatalf("pending %+v", pending)
	}
	if pending[1].Attempts != 0 || len(pending[1].Waiting) != 1 || pending[1].Waiting[0] != "messaging" {
		t.Fatalf("the later event %+v", pending[1])
	}

	fixture.pusher.Recover("messaging")
	fixture.now = startTime().Add(4 * time.Second)
	early := fixture.run(t)
	if early != (Report{Documents: 1}) || len(fixture.pusher.PushedTo("messaging")) != 0 {
		t.Fatalf("early report %+v, pushed %+v", early, fixture.pusher.Pushed())
	}

	fixture.now = startTime().Add(5 * time.Second)
	late := fixture.run(t)
	delivered := eventIds(fixture.pusher.PushedTo("messaging"))
	if late != (Report{Documents: 1, Delivered: 2}) || len(delivered) != 2 || delivered[0] != "evt_1" || delivered[1] != "evt_2" {
		t.Fatalf("late report %+v, delivered %v", late, delivered)
	}
	if fixture.store.Fields(fixture.ref).OutboxPending != 0 {
		t.Fatal("events are still pending")
	}
}

func TestRelayDeadLettersAPermanentFailureAtOnce(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))
	fixture.pusher.FailFor("messaging", DeliveryFailure{Subscriber: "messaging", Status: 400, Permanent: true, Detail: "400 Bad Request"})

	report := fixture.run(t)

	if report != (Report{Documents: 1, Delivered: 1, DeadLettered: 1}) {
		t.Fatalf("report %+v", report)
	}
	fields := fixture.store.Fields(fixture.ref)
	dead, err := DeadLetters{}.List(fields)
	if err != nil || fields.OutboxPending != 0 || fields.OutboxDead != 1 || dead[0].Subscriber != "messaging" || dead[0].Attempts != 1 {
		t.Fatalf("fields %+v, dead %+v, error %v", fields, dead, err)
	}
	if dead[0].DeadAt != "2026-11-10T12:00:00Z" || dead[0].LastError == "" {
		t.Fatalf("dead letter %+v", dead[0])
	}
}

func TestRelayDeadLettersAnEventWhenAttemptsRunOut(t *testing.T) {
	settings := testSettings()
	settings.Retry = RetrySettings{BaseDelaySeconds: 1, MaxAttempts: 2}
	fixture := newRelayFixture(t, settings, channelCreated("evt_1"))
	fixture.pusher.FailFor("messaging", errors.New("messaging is down"))

	first := fixture.run(t)
	fixture.now = startTime().Add(time.Second)
	second := fixture.run(t)

	if first != (Report{Documents: 1, Delivered: 1, Retried: 1}) || second != (Report{Documents: 1, Retried: 1, DeadLettered: 1}) {
		t.Fatalf("reports %+v and %+v", first, second)
	}
	fields := fixture.store.Fields(fixture.ref)
	dead, _ := DeadLetters{}.List(fields)
	if fields.OutboxPending != 0 || len(dead) != 1 || dead[0].Attempts != 2 || dead[0].LastError != "messaging is down" {
		t.Fatalf("fields %+v, dead %+v", fields, dead)
	}
}

func TestRelayPushesAgainWhenTheSaveFailed(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))
	fixture.store.FailNextSave(errors.New("the store stopped"))

	report, err := fixture.relay.RunOnce(10)
	if err == nil || report != (Report{Documents: 1, Delivered: 2}) {
		t.Fatalf("report %+v, error %v", report, err)
	}
	fixture.run(t)

	if len(fixture.pusher.PushedTo("messaging")) != 2 || fixture.store.Fields(fixture.ref).OutboxPending != 0 {
		t.Fatalf("pushed %+v", fixture.pusher.Pushed())
	}
}

func TestRelayKeepsAnEventStagedDuringTheRun(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))
	staged := false
	fixture.relay.pusher = hookPusher{inner: fixture.pusher, hook: func() {
		if staged {
			return
		}
		fields, _ := NewOutbox(testSettings()).Stage(fixture.store.Fields(fixture.ref), channelCreated("evt_2"))
		fixture.store.Put(fixture.ref, fields)
		staged = true
	}}

	fixture.run(t)

	pending := pendingOf(t, fixture.store.Fields(fixture.ref))
	if !staged || len(pending) != 1 || pending[0].Envelope.EventID != "evt_2" || len(pending[0].Waiting) != 2 {
		t.Fatalf("pending %+v", pending)
	}
}

func TestRelayReportsAFailingPendingQuery(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))
	fixture.store.FailNextPending(errors.New("query failed"))

	_, err := fixture.relay.RunOnce(10)

	if err == nil || len(fixture.pusher.Attempted()) != 0 {
		t.Fatalf("error %v", err)
	}
}

func TestRelayReportsADocumentItCannotRead(t *testing.T) {
	fixture := newRelayFixture(t, testSettings())
	fixture.store.Put(fixture.ref, Fields{PendingEvents: []string{"not json"}, OutboxPending: 1, OutboxWaiting: true})

	_, err := fixture.relay.RunOnce(10)

	if err == nil {
		t.Fatal("a broken document should be reported")
	}
}

func TestRelayReportsAFailingFreshLoad(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))
	fixture.store.FailNextLoad(errors.New("load failed"))

	report, err := fixture.relay.RunOnce(10)

	if err == nil || report.Delivered != 2 || fixture.store.Fields(fixture.ref).OutboxPending != 1 {
		t.Fatalf("report %+v, error %v", report, err)
	}
}

func TestRelayReportsAMissingDocument(t *testing.T) {
	fixture := newRelayFixture(t, testSettings())

	_, err := fixture.relay.RelayDocument(Ref{Collection: "channels", Id: "missing"})

	if !errors.As(err, &NotFound{}) {
		t.Fatalf("error %v", err)
	}
}

func TestRelayReportsAFreshDocumentItCannotMerge(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))
	fixture.relay.pusher = hookPusher{inner: fixture.pusher, hook: func() {
		fixture.store.Put(fixture.ref, Fields{DeadEvents: []string{"not json"}, OutboxPending: 1})
	}}

	_, err := fixture.relay.RunOnce(10)

	if err == nil {
		t.Fatal("a broken fresh document should be reported")
	}
}

func TestRelayKeepsSweepingAfterAFailedDocument(t *testing.T) {
	fixture := newRelayFixture(t, testSettings(), channelCreated("evt_1"))
	fixture.store.Put(Ref{Collection: "channels", Id: "chn_2"}, stageAll(t, testSettings(), channelCreated("evt_2")))
	fixture.store.FailNextSave(errors.New("the store stopped"))

	report, err := fixture.relay.RunOnce(10)

	if err == nil || report.Documents != 2 || len(fixture.pusher.Attempted()) != 4 {
		t.Fatalf("report %+v, error %v", report, err)
	}
	if fixture.store.Fields(Ref{Collection: "channels", Id: "chn_2"}).OutboxPending != 0 {
		t.Fatal("the second document should be delivered after the first failed")
	}
}
