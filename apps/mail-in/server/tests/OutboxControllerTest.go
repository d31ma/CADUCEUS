import (
	"mesail.dev/packages/mailin"
	"mesail.dev/packages/outbox"
)

// @Test(InboundTestSetup, {"partition":"tests"})
type OutboxControllerTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxControllerTest) TestARefusedEventBecomesADeadLetterThatTheRelayDeliversAfterARetry() {
	world := InboundTestSetup{}.World()
	defer world.Cleanup()
	world.Pusher.FailFor("messaging", outbox.DeliveryFailure{Subscriber: "messaging", Status: 400, Permanent: true, Detail: "bad"})
	key := world.FreshKey()
	_, err := world.Inbound.PostMessage(world.Envelope(key))
	world.Must(err)
	stored := world.Stored(key)
	dead := world.DeadFor(stored.ID)
	Assert.Equals(1, len(dead))
	Assert.Equals("messaging", dead[0].Subscriber)
	Assert.Equals("email.received", dead[0].Type)
	Assert.Equals(world.Collection(), dead[0].Collection)
	world.Pusher.Recover("messaging")
	retried := world.Outbox.PostRetry(RetryRequest{Collection: dead[0].Collection, DocumentId: dead[0].DocumentId, EventId: dead[0].EventId, Subscriber: "messaging"})
	_, relayed := world.Outbox.PostRelay()
	Assert.IsTrue(retried == nil, "retry failed")
	Assert.IsTrue(relayed == nil, "relay failed")
	Assert.Equals(1, len(world.PushedToMessaging()))
	Assert.Equals(0, world.Stored(key).OutboxPending)
	Assert.Equals(0, len(world.DeadFor(stored.ID)))
}

func (test *OutboxControllerTest) TestRetryOfAnUnknownCollectionIsNotFound() {
	world := InboundTestSetup{}.World()
	err := world.Outbox.PostRetry(RetryRequest{Collection: "inbound-raw-" + world.RandomText(), DocumentId: "1", EventId: "evt_1", Subscriber: "messaging"})
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *OutboxControllerTest) TestRetryOfAnUnknownLetterIsNotFound() {
	world := InboundTestSetup{}.World()
	defer world.Cleanup()
	key := world.FreshKey()
	_, err := world.Inbound.PostMessage(world.Envelope(key))
	world.Must(err)
	missing := world.Outbox.PostRetry(RetryRequest{Collection: world.Collection(), DocumentId: world.Stored(key).ID, EventId: "evt_missing", Subscriber: "messaging"})
	Assert.IsTrue(world.IsNotFound(missing), "want NotFound")
}

func (test *OutboxControllerTest) TestRetryOfAMissingDocumentIsNotFound() {
	world := InboundTestSetup{}.World()
	defer world.Cleanup()
	_, err := world.Workflows.Inbox().Collection(world.Domain)
	world.Must(err)
	missing := world.Outbox.PostRetry(RetryRequest{Collection: world.Collection(), DocumentId: "00000000000000", EventId: "evt_1", Subscriber: "messaging"})
	Assert.IsTrue(world.IsNotFound(missing), "want NotFound")
}

func (test *OutboxControllerTest) TestTheSweepSurvivesARegisteredCollectionThatIsMissing() {
	world := InboundTestSetup{}.World()
	registry := mailin.CollectionRegistry{Storage: world.Workflows.Inbox().Storage}
	missing := "inbound-raw-missing-" + world.RandomText()
	world.Must(registry.Register(missing))
	defer world.Forget(missing)
	_, err := world.Outbox.PostRelay()
	Assert.IsTrue(err == nil, "the sweep failed")
}

func (test *OutboxControllerTest) TestGetDeadIsAList() {
	world := InboundTestSetup{}.World()
	dead, err := world.Outbox.GetDead()
	Assert.IsTrue(err == nil, "listing failed")
	Assert.IsTrue(dead != nil, "want a list")
}
