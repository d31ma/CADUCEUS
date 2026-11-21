import (
	"encoding/json"
	"time"

	"mesail.dev/packages/channels"
	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Test(ChannelsTestSetup, {"partition":"tests"})
type OutboxDueOrderTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxDueOrderTest) TestADueDocumentIsSweptWhileManyLaterDocumentsWait() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	collection := test.registeredCollection(world)
	later := test.attemptTime(time.Hour)
	laterIds := test.insertManyWaiting(world, collection, later)
	dueId := test.insertWaiting(world, collection, test.attemptTime(-time.Minute))
	err := world.RelayUntilDone()
	Assert.IsTrue(err == nil, "relay failed")
	Assert.Equals(0, test.fieldsOf(world, collection, dueId).OutboxPending)
	Assert.Equals(test.laterCount(), test.unchangedCount(world, collection, laterIds, later))
}

func (test *OutboxDueOrderTest) registeredCollection(world *ChannelsTestSetup) string {
	collection := channels.Names{}.Channels(world.Domain)
	world.Must(test.documents().Ensure(collection))
	world.Must(channels.NewRegistry(test.documents()).Register(collection))
	return collection
}

func (test *OutboxDueOrderTest) insertManyWaiting(world *ChannelsTestSetup, collection string, nextAttemptAt string) []string {
	ids := []string{}
	for count := 0; count < test.laterCount(); count++ {
		ids = append(ids, test.insertWaiting(world, collection, nextAttemptAt))
	}
	return ids
}

func (test *OutboxDueOrderTest) insertWaiting(world *ChannelsTestSetup, collection string, nextAttemptAt string) string {
	event := outbox.PendingEvent{Envelope: test.envelope(world), Waiting: []string{test.subscriber()}, Attempts: test.firstAttempt(), NextAttemptAt: nextAttemptAt}
	fields, err := outbox.Codec{}.Write(outbox.OutboxState{Pending: []outbox.PendingEvent{event}, Dead: []outbox.DeadLetter{}})
	world.Must(err)
	id, err := test.documents().Insert(collection, channels.OutboxPatch{}.From(fields))
	world.Must(err)
	return id
}

func (test *OutboxDueOrderTest) envelope(world *ChannelsTestSetup) events.Envelope {
	return events.Envelope{
		EventID:    "evt_" + world.RandomText(),
		Type:       "channel.created",
		DomainId:   world.Domain,
		Producer:   "channels",
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:    json.RawMessage(`{}`),
	}
}

func (test *OutboxDueOrderTest) unchangedCount(world *ChannelsTestSetup, collection string, ids []string, nextAttemptAt string) int {
	unchanged := 0
	for _, id := range ids {
		unchanged += test.oneIfUnchanged(test.firstPending(world, collection, id), nextAttemptAt)
	}
	return unchanged
}

func (test *OutboxDueOrderTest) oneIfUnchanged(event outbox.PendingEvent, nextAttemptAt string) int {
	if event.Attempts == test.firstAttempt() && event.NextAttemptAt == nextAttemptAt {
		return 1
	}
	return 0
}

func (test *OutboxDueOrderTest) firstPending(world *ChannelsTestSetup, collection string, id string) outbox.PendingEvent {
	pending, err := outbox.Codec{}.DecodePending(test.fieldsOf(world, collection, id).PendingEvents)
	world.Must(err)
	return append(pending, outbox.PendingEvent{})[0]
}

func (test *OutboxDueOrderTest) fieldsOf(world *ChannelsTestSetup, collection string, id string) outbox.Fields {
	fields, err := channels.NewOutboxStore(test.documents()).Load(outbox.Ref{Collection: collection, Id: id})
	world.Must(err)
	return fields
}

func (test *OutboxDueOrderTest) documents() channels.CommandDocuments {
	return channels.NewCommandDocuments(DataSource{})
}

func (test *OutboxDueOrderTest) attemptTime(fromNow time.Duration) string {
	return time.Now().UTC().Add(fromNow).Format(time.RFC3339Nano)
}

func (test *OutboxDueOrderTest) laterCount() int {
	return 26
}

func (test *OutboxDueOrderTest) firstAttempt() int {
	return 1
}

func (test *OutboxDueOrderTest) subscriber() string {
	return "messaging"
}
