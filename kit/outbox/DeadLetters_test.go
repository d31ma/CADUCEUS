package outbox

import (
	"errors"
	"testing"
)

func deadFields(t *testing.T, pending []PendingEvent, dead []DeadLetter) Fields {
	t.Helper()
	fields, err := Codec{}.Write(OutboxState{Pending: pending, Dead: dead})
	if err != nil {
		t.Fatal(err)
	}
	return fields
}

func TestDeadLettersRequeueMovesALetterBackToPending(t *testing.T) {
	fields := deadFields(t, nil, []DeadLetter{
		{Envelope: channelCreated("evt_1"), Subscriber: "messaging", Attempts: 12},
		{Envelope: channelCreated("evt_1"), Subscriber: "realtime", Attempts: 12},
	})

	requeued, err := DeadLetters{}.Requeue(fields, "evt_1", "messaging")

	pending := pendingOf(t, requeued)
	dead, _ := DeadLetters{}.List(requeued)
	if err != nil || requeued.OutboxPending != 1 || requeued.OutboxDead != 1 || dead[0].Subscriber != "realtime" {
		t.Fatalf("fields %+v, error %v", requeued, err)
	}
	if pending[0].Attempts != 0 || len(pending[0].Waiting) != 1 || pending[0].Waiting[0] != "messaging" {
		t.Fatalf("pending %+v", pending)
	}
}

func TestDeadLettersRequeueJoinsAnEventThatIsStillPending(t *testing.T) {
	fields := deadFields(t,
		[]PendingEvent{{Envelope: channelCreated("evt_1"), Waiting: []string{"realtime"}, Attempts: 1}},
		[]DeadLetter{{Envelope: channelCreated("evt_1"), Subscriber: "messaging"}})

	requeued, err := DeadLetters{}.Requeue(fields, "evt_1", "messaging")

	pending := pendingOf(t, requeued)
	if err != nil || len(pending) != 1 || len(pending[0].Waiting) != 2 || pending[0].Waiting[1] != "messaging" || requeued.OutboxDead != 0 {
		t.Fatalf("pending %+v, error %v", pending, err)
	}
}

func TestDeadLettersRequeueReportsAMissingLetter(t *testing.T) {
	fields := deadFields(t, nil, []DeadLetter{{Envelope: channelCreated("evt_1"), Subscriber: "messaging"}})

	_, err := DeadLetters{}.Requeue(fields, "evt_1", "realtime")

	if !errors.As(err, &NotFound{}) || err.Error() != "the dead letter evt_1 for realtime was not found" {
		t.Fatalf("error %v", err)
	}
}

func TestDeadLettersRequeueReportsFieldsItCannotRead(t *testing.T) {
	_, err := DeadLetters{}.Requeue(Fields{DeadEvents: []string{"not json"}}, "evt_1", "messaging")

	if err == nil {
		t.Fatal("broken fields should be reported")
	}
}

func TestDeadLettersRequeuedEventIsDeliveredByTheRelay(t *testing.T) {
	fixture := newRelayFixture(t, testSettings())
	fixture.store.Put(fixture.ref, deadFields(t, nil, []DeadLetter{{Envelope: channelCreated("evt_1"), Subscriber: "messaging"}}))
	requeued, err := DeadLetters{}.Requeue(fixture.store.Fields(fixture.ref), "evt_1", "messaging")
	if err != nil {
		t.Fatal(err)
	}
	fixture.store.Put(fixture.ref, requeued)

	report := fixture.run(t)

	if report.Delivered != 1 || len(fixture.pusher.PushedTo("messaging")) != 1 {
		t.Fatalf("report %+v", report)
	}
}
