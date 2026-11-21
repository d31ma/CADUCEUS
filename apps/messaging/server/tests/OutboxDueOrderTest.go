import (
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Test(StorageTestSetup, {"partition":"tests"})
type OutboxDueOrderTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxDueOrderTest) TestTheSweepReachesADueDocumentBehindManyDocumentsThatAreNotDue() {
	world := StorageTestSetup{}.World()
	later := test.moment(time.Hour)
	notDueIds := test.waitingDocuments(world, later, test.notDueCount())
	dueId := test.waitingDocuments(world, test.moment(-time.Minute), 1)[0]
	err := world.RelayUntil(world.Relay.RunOnce, func() bool { return test.relayed(world, dueId) })
	Assert.IsTrue(err == nil, "the sweep failed")
	Assert.IsTrue(test.relayed(world, dueId), "the due document was not relayed")
	Assert.Equals(test.notDueCount(), test.untouchedCount(world, notDueIds, later))
}

func (test *OutboxDueOrderTest) waitingDocuments(world *StorageTestSetup, nextAttemptAt string, count int) []string {
	ids := []string{}
	for index := 0; index < count; index++ {
		ids = append(ids, test.waitingDocument(world, nextAttemptAt))
	}
	return ids
}

func (test *OutboxDueOrderTest) waitingDocument(world *StorageTestSetup, nextAttemptAt string) string {
	message := core.Message{DomainId: world.Domain, ConversationId: test.conversation(), Month: test.month(), Body: "waiting"}
	id, err := world.Messages().Insert(message)
	world.Must(err)
	fields, err := outbox.Codec{}.Write(outbox.OutboxState{Pending: []outbox.PendingEvent{test.retriedEvent(world, nextAttemptAt)}})
	world.Must(err)
	world.Must(world.OutboxStore().Save(test.ref(world, id), fields))
	return id
}

func (test *OutboxDueOrderTest) retriedEvent(world *StorageTestSetup, nextAttemptAt string) outbox.PendingEvent {
	payload := events.MessageEdited{ConversationId: test.conversation(), MessageId: "msg_waiting", MemberIds: []string{"ana"}}
	staged := world.Staged(core.OutboxFields{}, "message.edited", payload)
	pending, err := outbox.Codec{}.DecodePending(staged.PendingEvents)
	world.Must(err)
	event := pending[0]
	event.Waiting = []string{"search"}
	event.Attempts = test.firstAttempts()
	event.NextAttemptAt = nextAttemptAt
	return event
}

func (test *OutboxDueOrderTest) relayed(world *StorageTestSetup, id string) bool {
	fields, err := world.OutboxStore().Load(test.ref(world, id))
	world.Must(err)
	return fields.OutboxPending == 0
}

func (test *OutboxDueOrderTest) untouchedCount(world *StorageTestSetup, ids []string, nextAttemptAt string) int {
	count := 0
	for _, id := range ids {
		count += world.countIf(test.untouched(world, id, nextAttemptAt))
	}
	return count
}

func (test *OutboxDueOrderTest) untouched(world *StorageTestSetup, id string, nextAttemptAt string) bool {
	fields, err := world.OutboxStore().Load(test.ref(world, id))
	world.Must(err)
	pending, err := outbox.Codec{}.DecodePending(fields.PendingEvents)
	world.Must(err)
	return len(pending) == 1 && pending[0].Attempts == test.firstAttempts() && pending[0].NextAttemptAt == nextAttemptAt
}

func (test *OutboxDueOrderTest) ref(world *StorageTestSetup, id string) outbox.Ref {
	return outbox.Ref{Collection: world.Names.Messages(world.Domain, test.month()), Id: id}
}

func (test *OutboxDueOrderTest) moment(offset time.Duration) string {
	return time.Now().Add(offset).UTC().Format(time.RFC3339Nano)
}

func (test *OutboxDueOrderTest) notDueCount() int {
	return 26
}

func (test *OutboxDueOrderTest) firstAttempts() int {
	return 1
}

func (test *OutboxDueOrderTest) conversation() string {
	return "cnv_due"
}

func (test *OutboxDueOrderTest) month() string {
	return "202611"
}
