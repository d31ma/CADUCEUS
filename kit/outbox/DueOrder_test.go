package outbox

import (
	"strconv"
	"testing"
	"time"
)

func scheduledEvent(eventId string, subscriber string, nextAttemptAt string) PendingEvent {
	return PendingEvent{Envelope: channelCreated(eventId), Waiting: []string{subscriber}, Attempts: 1, NextAttemptAt: nextAttemptAt}
}

func scheduledFields(t *testing.T, pending ...PendingEvent) Fields {
	t.Helper()
	fields, err := Codec{}.Write(OutboxState{Pending: pending})
	if err != nil {
		t.Fatal(err)
	}
	return fields
}

func stampOf(moment time.Time) string {
	return moment.UTC().Format(time.RFC3339Nano)
}

func putNotDue(t *testing.T, store *MemoryStore, count int, now time.Time) {
	t.Helper()
	for index := range count {
		id := "later_" + strconv.Itoa(index)
		store.Put(Ref{Collection: "channels", Id: id}, scheduledFields(t, scheduledEvent("evt_"+id, "messaging", stampOf(now.Add(10*time.Minute)))))
	}
}

func newDueFixture() *relayFixture {
	fixture := &relayFixture{store: NewMemoryStore(), pusher: NewRecordingPusher(), now: startTime()}
	fixture.relay = NewRelay(fixture.store, fixture.pusher, testSettings(), func() time.Time { return fixture.now })
	return fixture
}

func TestASweepRelaysTheDueDocumentBehindThirtyThatAreNotDue(t *testing.T) {
	fixture := newDueFixture()
	putNotDue(t, fixture.store, 30, fixture.now)
	fixture.store.Put(Ref{Collection: "channels", Id: "due"}, scheduledFields(t, scheduledEvent("evt_due", "messaging", stampOf(fixture.now.Add(-time.Second)))))

	report, err := fixture.relay.RunWithin(25, time.Minute)

	if err != nil || report.More || report.Documents != 1 || report.Delivered != 1 {
		t.Fatalf("report %+v, error %v", report, err)
	}
	if ids := eventIds(fixture.pusher.PushedTo("messaging")); len(ids) != 1 || ids[0] != "evt_due" {
		t.Fatalf("pushed %v", ids)
	}
}

func TestASweepRelaysDueDocumentsOldestDueFirst(t *testing.T) {
	fixture := newDueFixture()
	fixture.store.Put(Ref{Collection: "channels", Id: "a"}, scheduledFields(t, scheduledEvent("evt_a", "messaging", stampOf(fixture.now.Add(-time.Minute)))))
	fixture.store.Put(Ref{Collection: "channels", Id: "b"}, scheduledFields(t, scheduledEvent("evt_b", "messaging", stampOf(fixture.now.Add(-3*time.Minute)))))
	fixture.store.Put(Ref{Collection: "channels", Id: "c"}, scheduledFields(t, scheduledEvent("evt_c", "messaging", stampOf(fixture.now.Add(-2*time.Minute)))))

	_, err := fixture.relay.RunOnce(10)

	if ids := eventIds(fixture.pusher.PushedTo("messaging")); err != nil || len(ids) != 3 || ids[0] != "evt_b" || ids[1] != "evt_c" || ids[2] != "evt_a" {
		t.Fatalf("pushed %v, error %v", ids, err)
	}
}

func TestASweepOverDocumentsThatAreNotDueSaysNoMoreAndSavesNothing(t *testing.T) {
	fixture := newDueFixture()
	putNotDue(t, fixture.store, 30, fixture.now)

	report, err := fixture.relay.RunWithin(25, time.Minute)

	if err != nil || report != (Report{}) || fixture.store.Saves() != 0 || len(fixture.pusher.Attempted()) != 0 {
		t.Fatalf("report %+v, saves %d, error %v", report, fixture.store.Saves(), err)
	}
}

func TestASweepSaysMoreOnlyWhenDueDocumentsAreLeft(t *testing.T) {
	fixture := newDueFixture()
	putNotDue(t, fixture.store, 5, fixture.now)
	fixture.store.Put(Ref{Collection: "channels", Id: "due_1"}, scheduledFields(t, scheduledEvent("evt_1", "messaging", "")))
	fixture.store.Put(Ref{Collection: "channels", Id: "due_2"}, scheduledFields(t, scheduledEvent("evt_2", "messaging", "")))

	first, _ := fixture.relay.RunOnce(1)
	second, _ := fixture.relay.RunOnce(1)

	if !first.More || first.Documents != 1 || second.More || second.Documents != 1 {
		t.Fatalf("reports %+v and %+v", first, second)
	}
}

func TestADocumentWithOneDueAndOneNotDueEventPushesOnlyTheDueEvent(t *testing.T) {
	fixture := newDueFixture()
	ref := Ref{Collection: "channels", Id: "mixed"}
	fixture.store.Put(ref, scheduledFields(t,
		scheduledEvent("evt_later", "messaging", stampOf(fixture.now.Add(time.Minute))),
		scheduledEvent("evt_now", "realtime", stampOf(fixture.now.Add(-time.Minute))),
	))

	report, err := fixture.relay.RunOnce(10)

	if err != nil || report.Documents != 1 || len(fixture.pusher.PushedTo("messaging")) != 0 || len(fixture.pusher.PushedTo("realtime")) != 1 {
		t.Fatalf("report %+v, pushed %+v, error %v", report, fixture.pusher.Pushed(), err)
	}
	if fixture.store.Fields(ref).OutboxPending != 1 {
		t.Fatalf("fields %+v", fixture.store.Fields(ref))
	}
}

func TestANewEventBehindANotDueEventForTheSameSubscriberIsNotDue(t *testing.T) {
	fixture := newDueFixture()
	fixture.store.Put(Ref{Collection: "channels", Id: "blocked"}, scheduledFields(t,
		scheduledEvent("evt_waiting", "messaging", stampOf(fixture.now.Add(time.Minute))),
		scheduledEvent("evt_new", "messaging", ""),
	))
	fixture.store.Put(Ref{Collection: "channels", Id: "due"}, scheduledFields(t, scheduledEvent("evt_due", "messaging", stampOf(fixture.now.Add(-time.Minute)))))

	report, err := fixture.relay.RunOnce(1)

	if ids := eventIds(fixture.pusher.PushedTo("messaging")); err != nil || report.More || len(ids) != 1 || ids[0] != "evt_due" {
		t.Fatalf("report %+v, pushed %v, error %v", report, ids, err)
	}
}

func TestADocumentThatCannotBeReadComesAfterTheDueDocuments(t *testing.T) {
	fixture := newDueFixture()
	fixture.store.Put(Ref{Collection: "channels", Id: "broken"}, Fields{PendingEvents: []string{"not json"}, OutboxPending: 1, OutboxWaiting: true})
	fixture.store.Put(Ref{Collection: "channels", Id: "due"}, scheduledFields(t, scheduledEvent("evt_due", "messaging", "")))

	report, err := fixture.relay.RunOnce(1)

	if ids := eventIds(fixture.pusher.PushedTo("messaging")); err != nil || !report.More || len(ids) != 1 || ids[0] != "evt_due" {
		t.Fatalf("report %+v, pushed %v, error %v", report, ids, err)
	}
}

func TestDueOrderBreaksTiesByCollectionAndId(t *testing.T) {
	at := stampOf(startTime().Add(-time.Minute))
	documents := []Document{
		{Ref: Ref{Collection: "members", Id: "1"}, Fields: scheduledFields(t, scheduledEvent("evt_1", "messaging", at))},
		{Ref: Ref{Collection: "channels", Id: "2"}, Fields: scheduledFields(t, scheduledEvent("evt_2", "messaging", at))},
		{Ref: Ref{Collection: "channels", Id: "1"}, Fields: scheduledFields(t, scheduledEvent("evt_3", "messaging", at))},
	}

	due := DueOrder{}.Due(documents, startTime())

	if len(due) != 3 || due[0].Ref.Id != "1" || due[0].Ref.Collection != "channels" || due[1].Ref.Id != "2" || due[2].Ref.Collection != "members" {
		t.Fatalf("order %+v", due)
	}
}
