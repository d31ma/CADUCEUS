import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/mailin"
	"mesail.dev/packages/outbox"
)

// @Test(InboundTestSetup, {"partition":"tests"})
type OutboxDueSweepTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxDueSweepTest) TestTheSweepAttemptsADueDocumentBehindManyThatAreNotDue() {
	world := InboundTestSetup{}.World()
	defer world.Cleanup()
	collection, err := world.Workflows.Inbox().Collection(world.Domain)
	world.Must(err)
	later := test.moment(time.Hour)
	notDue := test.waitingDocuments(world, collection, later, test.notDueCount())
	due := test.waitingDocument(world, collection, test.moment(-time.Minute))
	relayed := world.RelayUntil(world.Outbox.PostRelay, func() bool { return test.stored(world, collection, due).OutboxPending == 0 })
	Assert.IsTrue(relayed == nil, "relay failed")
	Assert.Equals(0, test.stored(world, collection, due).OutboxPending)
	Assert.Equals(test.unchanged(later, test.notDueCount()), test.pendingOf(world, collection, notDue))
}

func (test *OutboxDueSweepTest) notDueCount() int {
	return 26
}

func (test *OutboxDueSweepTest) subscriber() string {
	return "messaging"
}

func (test *OutboxDueSweepTest) moment(offset time.Duration) string {
	return time.Now().UTC().Add(offset).Format(time.RFC3339Nano)
}

func (test *OutboxDueSweepTest) waitingDocuments(world *InboundTestSetup, collection string, nextAttemptAt string, count int) []string {
	ids := []string{}
	for range count {
		ids = append(ids, test.waitingDocument(world, collection, nextAttemptAt))
	}
	return ids
}

func (test *OutboxDueSweepTest) waitingDocument(world *InboundTestSetup, collection string, nextAttemptAt string) string {
	pending := outbox.PendingEvent{Envelope: test.envelope(world), Waiting: []string{test.subscriber()}, Attempts: 1, NextAttemptAt: nextAttemptAt}
	fields, err := outbox.Codec{}.Write(outbox.OutboxState{Pending: []outbox.PendingEvent{pending}, Dead: []outbox.DeadLetter{}})
	world.Must(err)
	record := mailin.InboundRaw{DomainId: world.Domain, ReceivedAt: test.moment(0), RawBlobKey: world.FreshKey(), State: mailin.Inbox{}.AcceptedState(), EventQueued: true, Fields: fields}
	id, err := world.Workflows.Inbox().Storage.Insert(collection, record)
	world.Must(err)
	return id
}

func (test *OutboxDueSweepTest) envelope(world *InboundTestSetup) events.Envelope {
	return events.Envelope{
		EventID:    "evt_" + world.RandomText(),
		Type:       events.Types{}.EmailReceived(),
		DomainId:   world.Domain,
		Producer:   mailin.EventStager{}.Producer(),
		OccurredAt: test.moment(0),
		Payload:    json.RawMessage("{}"),
	}
}

func (test *OutboxDueSweepTest) stored(world *InboundTestSetup, collection string, id string) mailin.InboundRaw {
	raw := mailin.InboundRaw{}
	_, err := world.Workflows.Inbox().Storage.Find(collection, id, &raw)
	world.Must(err)
	return raw
}

func (test *OutboxDueSweepTest) pendingOf(world *InboundTestSetup, collection string, ids []string) string {
	summaries := []string{}
	for _, id := range ids {
		summaries = append(summaries, test.summary(world, test.stored(world, collection, id)))
	}
	return strings.Join(summaries, test.lineBreak())
}

func (test *OutboxDueSweepTest) summary(world *InboundTestSetup, raw mailin.InboundRaw) string {
	pending, err := outbox.Codec{}.DecodePending(raw.PendingEvents)
	world.Must(err)
	first := append(pending, outbox.PendingEvent{})[0]
	return fmt.Sprintf(test.summaryFormat(), len(pending), first.Attempts, first.NextAttemptAt)
}

func (test *OutboxDueSweepTest) unchanged(nextAttemptAt string, count int) string {
	summaries := []string{}
	for range count {
		summaries = append(summaries, fmt.Sprintf(test.summaryFormat(), 1, 1, nextAttemptAt))
	}
	return strings.Join(summaries, test.lineBreak())
}

func (test *OutboxDueSweepTest) summaryFormat() string {
	return "%d pending, attempts %d, next attempt at %s"
}

func (test *OutboxDueSweepTest) lineBreak() string {
	return "\n"
}
