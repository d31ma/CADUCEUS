import (
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

// @Test(StorageTestSetup, {"partition":"tests"})
type ChangeLogTest struct {
	behaviors *BehaviorContext
}

func (test *ChangeLogTest) TestTheRelayLogsAChangeBeforeItPushes() {
	world := StorageTestSetup{}.World()
	conversationId := "cnv_" + world.RandomText()
	message := core.Message{DomainId: world.Domain, ConversationId: conversationId, Month: "202611", Body: "hi"}
	message.OutboxFields = world.Staged(message.OutboxFields, "message.edited", events.MessageEdited{ConversationId: conversationId, MessageId: "msg_202611" + world.RandomText(), MemberIds: []string{"ana"}})
	_, err := world.Messages().Insert(message)
	world.Must(err)
	err = world.RelayUntilDone(world.Relay.RunOnce)
	world.Must(err)
	rows, err := world.Stores().Changes.OfConversation(world.Domain, core.MessageIds{}.Month(time.Now()), conversationId)
	world.Must(err)
	head, found, err := world.Stores().ChangeHeads.Find(world.Domain, conversationId)
	world.Must(err)
	Assert.Equals(1, len(rows))
	Assert.IsTrue(found && head.LastChangeId != "", "the head is missing")
	Assert.Equals(1, world.Pushed("realtime", "message.edited"))
}

func (test *ChangeLogTest) TestTheCleanupDropsMonthsOlderThanTheLogKeeps() {
	world := StorageTestSetup{}.World()
	old := world.Names.Changes(world.Domain, "202001")
	_, err := world.Stores().Changes.Append(core.Change{DomainId: world.Domain, ConversationId: "cnv_1", MessageId: "msg_1", EventId: "e1", Month: "202001"})
	world.Must(err)
	controller := &JobsController{changes: &ChangeJobService{storage: world.Storage, failures: &FailureService{}, wire: &WireService{}}}
	report := world.CleanedUp(controller)
	exists, err := DataSource{}.CollectionExists(old)
	world.Must(err)
	Assert.IsTrue(report.Dropped >= 1 && !report.More, "want the old month dropped")
	Assert.IsTrue(!exists, "the old month is still there")
}

func (test *ChangeLogTest) TestADomainTokenCannotRunTheCleanup() {
	world := StorageTestSetup{}.World()
	controller := &JobsController{changes: &ChangeJobService{storage: world.Storage, failures: &FailureService{}, wire: &WireService{}}}
	_, err := controller.PostChangesCleanup(world.Bearer(world.Home))
	Assert.IsTrue(world.IsForbidden(err), "want Forbidden")
	Assert.Equals(1566, world.Number(err))
}
