import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"mesail.dev/packages/devicebox"
	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Test(DeviceboxTestSetup, {"partition":"tests"})
type OutboxDueTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxDueTest) TestADueDocumentBehindManyNotDueDocumentsIsRelayed() {
	world := DeviceboxTestSetup{}.World()
	defer world.Close()
	world.Owns("ana", "a1")
	world.Owns("bo", "b1")
	world.Put("ana", "a1", "b1")
	later := time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
	notDue := test.putMany(world, test.notDueCount(), later)
	due := test.putWaiting(world, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano))
	err := test.relayUntilDelivered(world, due)
	Assert.IsTrue(err == nil, fmt.Sprint("relay failed: ", err))
	Assert.Equals(0, test.load(world, due).OutboxPending)
	Assert.Equals(test.unchanged(test.notDueCount(), later), test.states(world, notDue))
}

func (test *OutboxDueTest) putMany(world *DeviceboxTestSetup, count int, nextAttemptAt string) []outbox.Ref {
	refs := []outbox.Ref{}
	for index := 0; index < count; index++ {
		refs = append(refs, test.putWaiting(world, nextAttemptAt))
	}
	return refs
}

func (test *OutboxDueTest) putWaiting(world *DeviceboxTestSetup, nextAttemptAt string) outbox.Ref {
	fields, err := outbox.Codec{}.Write(outbox.OutboxState{Pending: []outbox.PendingEvent{test.event(world, nextAttemptAt)}})
	world.Must(err)
	collection := devicebox.Names{}.Bodies(world.Domain)
	id, err := devicebox.NewDocuments(world.Storage).Insert(collection, devicebox.OutboxPatch{}.From(fields))
	world.Must(err)
	return outbox.Ref{Collection: collection, Id: id}
}

func (test *OutboxDueTest) event(world *DeviceboxTestSetup, nextAttemptAt string) outbox.PendingEvent {
	envelope := events.Envelope{
		EventID:    "evt_" + world.RandomText(),
		Type:       events.Types{}.EnvelopeStored(),
		DomainId:   world.Domain,
		Producer:   "devicebox",
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:    json.RawMessage(`{}`),
	}
	return outbox.PendingEvent{Envelope: envelope, Waiting: []string{test.subscriber()}, Attempts: 1, NextAttemptAt: nextAttemptAt}
}

func (test *OutboxDueTest) relayUntilDelivered(world *DeviceboxTestSetup, ref outbox.Ref) error {
	failures := []error{}
	for round := 0; round < test.rounds() && test.load(world, ref).OutboxPending > 0; round++ {
		_, failure := world.Outbox.PostRelay()
		failures = append(failures, failure)
	}
	return errors.Join(failures...)
}

func (test *OutboxDueTest) load(world *DeviceboxTestSetup, ref outbox.Ref) outbox.Fields {
	fields, err := devicebox.NewOutboxStore(devicebox.NewDocuments(world.Storage)).Load(ref)
	world.Must(err)
	return fields
}

func (test *OutboxDueTest) states(world *DeviceboxTestSetup, refs []outbox.Ref) []string {
	states := []string{}
	for _, ref := range refs {
		state, err := outbox.Codec{}.Read(test.load(world, ref))
		world.Must(err)
		states = append(states, test.stateText(state.Pending))
	}
	return states
}

func (test *OutboxDueTest) unchanged(count int, nextAttemptAt string) []string {
	states := []string{}
	for index := 0; index < count; index++ {
		states = append(states, test.stateText([]outbox.PendingEvent{{Attempts: 1, NextAttemptAt: nextAttemptAt}}))
	}
	return states
}

func (test *OutboxDueTest) stateText(pending []outbox.PendingEvent) string {
	texts := []string{}
	for _, event := range pending {
		texts = append(texts, fmt.Sprint(event.Attempts, " ", event.NextAttemptAt))
	}
	return fmt.Sprint(texts)
}

func (*OutboxDueTest) subscriber() string {
	return "realtime"
}

func (*OutboxDueTest) notDueCount() int {
	return 26
}

func (*OutboxDueTest) rounds() int {
	return 10
}
