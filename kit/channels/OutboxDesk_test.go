package channels

import (
	"errors"
	"testing"
	"time"

	"mesail.dev/packages/outbox"
)

func permanentFailure() outbox.DeliveryFailure {
	return outbox.DeliveryFailure{Subscriber: "search", Status: 410, Permanent: true, Detail: "gone"}
}

func TestARefusedEventBecomesADeadLetterThatCanBeRetried(t *testing.T) {
	w := newWorld(t)
	w.pusher.FailFor("search", permanentFailure())
	general := w.create("Bearer ana", "general", "public")
	dead, err := w.desk.DeadEvents()
	wantNoError(t, err)
	wantEqual(t, len(dead), 2)
	wantEqual(t, dead[0].Collection, Names{}.Channels("example.com"))
	wantEqual(t, dead[0].DocumentId, general.Id)
	wantEqual(t, dead[0].Type, "channel.created")
	wantEqual(t, dead[0].Subscriber, "search")
	wantEqual(t, dead[0].Attempts, 1)
	w.pusher.Recover("search")
	wantNoError(t, w.desk.RetryDead(dead[0].Collection, dead[0].DocumentId, dead[0].EventId, "search"))
	report, err := w.desk.RunOnce()
	wantNoError(t, err)
	wantEqual(t, report.Delivered, 1)
	wantEqual(t, w.types("search"), []string{"channel.created"})
	remaining, err := w.desk.DeadEvents()
	wantNoError(t, err)
	wantEqual(t, len(remaining), 1)
}

func TestTheSweepRetriesAWaitingEventAfterTheBackoff(t *testing.T) {
	w := newWorld(t)
	w.pusher.FailFor("messaging", errors.New("messaging is down"))
	w.create("Bearer ana", "general", "public")
	waiting, err := w.desk.Waiting()
	wantNoError(t, err)
	wantEqual(t, waiting, 2)
	w.pusher.Recover("messaging")
	early, err := w.desk.RunOnce()
	wantNoError(t, err)
	wantEqual(t, early.Delivered, 0)
	w.clock = w.clock.Add(time.Minute)
	late, err := w.desk.RunOnce()
	wantNoError(t, err)
	wantEqual(t, late.Delivered, 2)
	waiting, err = w.desk.Waiting()
	wantNoError(t, err)
	wantEqual(t, waiting, 0)
}

func TestRetryOfAnUnknownCollectionIsNotFound(t *testing.T) {
	w := newWorld(t)
	wantError[*NotFound](t, w.desk.RetryDead("no-such-collection", "1", "evt_1", "search"))
}

func TestRetryOfAnUnknownLetterIsNotFound(t *testing.T) {
	w := newWorld(t)
	general := w.create("Bearer ana", "general", "public")
	wantError[*NotFound](t, w.desk.RetryDead(Names{}.Channels("example.com"), general.Id, "evt_unknown", "search"))
}

func TestRetryOfAnUnknownDocumentIsNotFound(t *testing.T) {
	w := newWorld(t)
	w.create("Bearer ana", "general", "public")
	wantError[*NotFound](t, w.desk.RetryDead(Names{}.Channels("example.com"), "99", "evt_1", "search"))
}

func TestAnUnreadableOutboxIsReportedByTheDesk(t *testing.T) {
	w := newWorld(t)
	w.must(NewRegistry(w.documents).Register("broken"))
	w.must(w.documents.Ensure("broken"))
	id, err := w.documents.Insert("broken", OutboxRow{PendingEvents: []string{"{"}, DeadEvents: []string{"{"}, OutboxPending: 1, OutboxDead: 1, OutboxWaiting: true, OutboxHasDead: true})
	w.must(err)
	_, deadErr := w.desk.DeadEvents()
	retryErr := w.desk.RetryDead("broken", id, "evt_1", "search")
	wantTrue(t, deadErr != nil, "an unreadable dead letter was listed")
	wantError[*Unavailable](t, retryErr)
}

func TestLoadingAMissingDocumentIsNotFound(t *testing.T) {
	w := newWorld(t)
	w.must(NewRegistry(w.documents).Register("notes"))
	w.must(w.documents.Ensure("notes"))
	_, err := NewOutboxStore(w.documents).Load(outbox.Ref{Collection: "notes", Id: "1"})
	wantError[*NotFound](t, err)
}

func TestSavingToAnUnknownCollectionIsNotFound(t *testing.T) {
	w := newWorld(t)
	err := NewOutboxStore(w.documents).Save(outbox.Ref{Collection: "notes", Id: "1"}, outbox.Fields{})
	wantError[*NotFound](t, err)
}

func TestTheRegistryRefusesAnUnknownCollectionAndKeepsEachNameOnce(t *testing.T) {
	registry := NewRegistry(NewMemoryDocuments())
	wantError[*NotFound](t, registry.Check("notes"))
	wantNoError(t, registry.Register("notes"))
	wantNoError(t, registry.Register("notes"))
	wantNoError(t, registry.Check("notes"))
	names, err := registry.All()
	wantNoError(t, err)
	wantEqual(t, names, []string{"notes"})
}

func TestPendingReturnsAtMostTheLimit(t *testing.T) {
	w := newWorld(t)
	w.pusher.FailFor("messaging", errors.New("messaging is down"))
	w.create("Bearer ana", "general", "public")
	pending, err := NewOutboxStore(w.documents).Pending(1)
	wantNoError(t, err)
	wantEqual(t, len(pending), 1)
}

func TestARegisteredCollectionThatIsMissingFailsTheSweep(t *testing.T) {
	w := newWorld(t)
	w.must(NewRegistry(w.documents).Register("ghost"))
	_, pendingErr := NewOutboxStore(w.documents).Pending(10)
	_, runErr := w.desk.RunOnce()
	wantTrue(t, pendingErr != nil, "a missing collection was read")
	wantTrue(t, runErr != nil, "the sweep ignored a missing collection")
}

func TestADocumentFlaggedWithoutEventsIsNotPendingOrDead(t *testing.T) {
	w := newWorld(t)
	w.must(NewRegistry(w.documents).Register("notes"))
	w.must(w.documents.Ensure("notes"))
	_, err := w.documents.Insert("notes", OutboxRow{OutboxWaiting: true, OutboxHasDead: true})
	w.must(err)
	pending, pendingErr := NewOutboxStore(w.documents).Pending(10)
	dead, deadErr := NewOutboxStore(w.documents).Dead()
	wantNoError(t, pendingErr)
	wantNoError(t, deadErr)
	wantEqual(t, len(pending), 0)
	wantEqual(t, len(dead), 0)
}

func TestTheOutboxPatchFlagsWaitingAndDeadEvents(t *testing.T) {
	quiet := OutboxPatch{}.From(outbox.Fields{})
	busy := OutboxPatch{}.From(outbox.Fields{PendingEvents: []string{"a"}, DeadEvents: []string{"b"}, OutboxPending: 1, OutboxDead: 1})
	wantEqual(t, []bool{quiet.OutboxWaiting, quiet.OutboxHasDead}, []bool{false, false})
	wantEqual(t, []bool{busy.OutboxWaiting, busy.OutboxHasDead}, []bool{true, true})
	wantEqual(t, busy.PendingEvents, []string{"a"})
	wantEqual(t, busy.DeadEvents, []string{"b"})
}

func TestAnUndeliveredEventFlagsItsDocumentAsWaiting(t *testing.T) {
	w := newWorld(t)
	w.pusher.FailFor("messaging", errors.New("messaging is down"))
	general := w.create("Bearer ana", "general", "public")
	stored := w.stored("example.com", general)
	wantEqual(t, stored.OutboxWaiting, true)
	wantEqual(t, stored.OutboxHasDead, false)
}
