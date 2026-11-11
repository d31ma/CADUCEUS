import (
	"errors"

	"mesail.dev/packages/channels"
	"mesail.dev/packages/outbox"
)

// @Test(ChannelsTestSetup, {"partition":"tests"})
type OutboxTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxTest) TestARefusedEventBecomesADeadLetterThatCanBeRetried() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	world.Pusher.FailFor("search", outbox.DeliveryFailure{Subscriber: "search", Status: 400, Permanent: true, Detail: "bad"})
	created := world.Create(world.Member("ana"), "releases", "public")
	dead := test.deadFor(world, created.ChannelId[len("ch_"):])
	Assert.Equals(1, len(dead))
	Assert.Equals("channel.created", dead[0].Type)
	world.Pusher.Recover("search")
	err := world.Outbox.PostRetry(RetryRequest{Collection: dead[0].Collection, DocumentId: dead[0].DocumentId, EventId: dead[0].EventId, Subscriber: "search"})
	_, relayError := world.Outbox.PostRelay()
	Assert.IsTrue(err == nil, "retry failed")
	Assert.IsTrue(relayError == nil, "relay failed")
	Assert.Equals(1, len(world.Published("search", "channel.created", created.ChannelId)))
}

func (test *OutboxTest) TestPendingCountsTheDocumentsThatWait() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	world.Pusher.FailFor("search", errors.New("connection refused"))
	world.Create(world.Member("ana"), "releases", "public")
	pending, err := world.Outbox.GetPending()
	Assert.IsTrue(err == nil, "pending failed")
	Assert.IsTrue(pending.Documents >= 2, "the channel and its creator membership should wait")
}

func (test *OutboxTest) TestRetryOfAnUnknownCollectionIsNotFound() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	err := world.Outbox.PostRetry(RetryRequest{Collection: "members-" + world.RandomText(), DocumentId: "1", EventId: "evt_1", Subscriber: "search"})
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *OutboxTest) deadFor(world *ChannelsTestSetup, documentId string) []channels.DeadEvent {
	dead, err := world.Outbox.GetDead()
	world.Must(err)
	kept := []channels.DeadEvent{}
	for _, event := range dead {
		kept = append(kept, test.ifDocument(event, documentId)...)
	}
	return kept
}

func (test *OutboxTest) ifDocument(event channels.DeadEvent, documentId string) []channels.DeadEvent {
	if event.DocumentId == documentId {
		return []channels.DeadEvent{event}
	}
	return []channels.DeadEvent{}
}
