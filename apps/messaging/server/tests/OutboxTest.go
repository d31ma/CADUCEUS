import (
	"os"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Test(StorageTestSetup, {"partition":"tests"})
type OutboxTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxTest) TestARefusedEventBecomesADeadLetterThatTheOperatorCanRequeue() {
	world := StorageTestSetup{}.World()
	world.Pusher.FailFor("search", outbox.DeliveryFailure{Subscriber: "search", Status: 400, Permanent: true, Detail: "bad"})
	id := test.stagedMessage(world)
	controller := &OutboxController{relay: world.Relay}
	operator := world.Bearer(world.Operators)
	dead := test.deadFor(world, controller, operator, id)
	Assert.Equals(1, len(dead))
	Assert.Equals("message.edited", dead[0].Type)
	world.Pusher.Recover("search")
	err := controller.PostRetry(operator, RetryRequest{Collection: dead[0].Collection, DocumentId: id, EventId: dead[0].EventId, Subscriber: "search"})
	Assert.IsTrue(err == nil, "the requeue failed")
	_, err = controller.PostRelay(operator)
	Assert.IsTrue(err == nil, "the relay failed")
	Assert.Equals(1, world.Pushed("search", "message.edited"))
	Assert.Equals(0, len(test.deadFor(world, controller, operator, id)))
}

func (test *OutboxTest) TestADomainServiceTokenCannotUseTheOutboxRoutes() {
	world := StorageTestSetup{}.World()
	controller := &OutboxController{relay: world.Relay}
	domain := world.Bearer(world.Home)
	_, relayError := controller.PostRelay(domain)
	_, deadError := controller.GetDead(domain)
	retryError := controller.PostRetry(domain, RetryRequest{})
	Assert.IsTrue(world.IsForbidden(relayError), "relay must be refused")
	Assert.IsTrue(world.IsForbidden(deadError), "dead must be refused")
	Assert.IsTrue(world.IsForbidden(retryError), "requeue must be refused")
}

func (test *OutboxTest) TestTheSweepRetriesAfterAFailure() {
	world := StorageTestSetup{}.World()
	world.Pusher.FailFor("realtime", outbox.DeliveryFailure{Subscriber: "realtime", Detail: "down"})
	id := test.stagedMessage(world)
	_, err := world.Relay.RunOnce()
	Assert.IsTrue(err == nil, "the sweep failed")
	message, _, _ := world.Messages().Find(world.Domain, "202611", id)
	Assert.Equals(1, message.OutboxPending)
	Assert.Equals(1, world.Pushed("search", "message.edited"))
}

func (test *OutboxTest) TestUnknownCollectionsLettersAndDocumentsAreNotFound() {
	world := StorageTestSetup{}.World()
	id := test.stagedMessage(world)
	collection := world.Names.Messages(world.Domain, "202611")
	unknownCollection := world.Relay.RetryDead(RetryRequest{Collection: "messages-" + world.RandomText(), DocumentId: id, EventId: "evt_1", Subscriber: "search"})
	unknownLetter := world.Relay.RetryDead(RetryRequest{Collection: collection, DocumentId: id, EventId: "evt_missing", Subscriber: "search"})
	store := &OutboxStoreService{documents: world.Documents, registry: world.Registry}
	_, missing := store.Load(outbox.Ref{Collection: collection, Id: "00000000000000"})
	Assert.IsTrue(world.IsNotFound(unknownCollection), "want NotFound for the collection")
	Assert.IsTrue(world.IsNotFound(unknownLetter), "want NotFound for the letter")
	Assert.IsTrue(world.IsNotFound(missing), "want NotFound for the document")
	Assert.IsTrue(world.IsNotFound(world.Registry.Check("not-a-collection")), "want NotFound from the registry")
	Assert.IsTrue(world.IsNotFound(store.Save(outbox.Ref{Collection: "not-a-collection", Id: id}, outbox.Fields{})), "want NotFound on save")
}

func (test *OutboxTest) TestMissingOutboxSettingsAreUnavailable() {
	world := StorageTestSetup{}.World()
	os.Setenv("YON_APPLICATION_JSON", `{"yon":{}}`)
	_, settingsError := world.Settings.Outbox()
	_, runError := world.Relay.RunOnce()
	world.Relay.Drain(world.Names.Messages(world.Domain, "202611"), "00000000000000")
	Assert.IsTrue(world.IsUnavailable(settingsError), "want Unavailable settings")
	Assert.IsTrue(world.IsUnavailable(runError), "want Unavailable relay")
	Assert.Equals("", world.Settings.Settings().RelayUrl)
}

func (test *OutboxTest) stagedMessage(world *StorageTestSetup) string {
	message := core.Message{DomainId: world.Domain, ConversationId: "cnv_1", Month: "202611", Body: "hi"}
	message.OutboxFields = world.Staged(message.OutboxFields, "message.edited", events.MessageEdited{ConversationId: "cnv_1", MessageId: "msg_1", MemberIds: []string{"ana"}})
	id, err := world.Messages().Insert(message)
	world.Must(err)
	return id
}

func (test *OutboxTest) deadFor(world *StorageTestSetup, controller *OutboxController, authorization string, documentId string) []DeadEvent {
	dead, err := controller.GetDead(authorization)
	world.Must(err)
	kept := []DeadEvent{}
	for _, event := range dead {
		kept = append(kept, test.ifDocument(event, documentId)...)
	}
	return kept
}

func (test *OutboxTest) ifDocument(event DeadEvent, documentId string) []DeadEvent {
	if event.DocumentId == documentId {
		return []DeadEvent{event}
	}
	return []DeadEvent{}
}
