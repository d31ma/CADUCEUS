import (
	"errors"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Test(InboundTestSetup, {"partition":"tests"})
type OutboxTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxTest) TestAcceptStagesOneEmailReceivedEvent() {
	world := InboundTestSetup{}.World()
	world.Pusher.FailFor("messaging", errors.New("connection refused"))
	key := world.FreshKey()
	accepted, err := world.Inbound.Accept(world.Envelope(key))
	world.Must(err)
	stored := world.Stored(key)
	payload := events.EmailReceived{}
	envelope, staged := outbox.Published{}.One(world.Fields(stored), events.Types{}.EmailReceived(), &payload)
	Assert.IsTrue(staged == nil, "no single staged event")
	Assert.Equals(world.Domain, envelope.DomainId)
	Assert.Equals("mail-in", envelope.Producer)
	Assert.Equals(accepted.InboundId, payload.InboundId)
	Assert.Equals("ana@"+world.Domain, payload.Rcpts[0])
	Assert.Equals(key, payload.RawBlobKey)
	Assert.Equals("dan@outside.test", payload.Headers.From)
	Assert.Equals("Hello", payload.Headers.Subject)
	Assert.IsTrue(payload.Headers.To != nil && len(payload.Headers.To) == 0, "to must be an empty list")
	Assert.IsTrue(payload.Headers.Cc != nil && len(payload.Headers.Cc) == 0, "cc must be an empty list")
	Assert.Equals("pass", payload.AuthResults.Dmarc)
	Assert.Equals(1, stored.OutboxPending)
	Assert.IsTrue(stored.EventQueued, "event not marked")
}

func (test *OutboxTest) TestARetriedAcceptStagesNoSecondEvent() {
	world := InboundTestSetup{}.World()
	world.Pusher.FailFor("messaging", errors.New("connection refused"))
	key := world.FreshKey()
	_, first := world.Inbound.Accept(world.Envelope(key))
	_, second := world.Inbound.Accept(world.Envelope(key))
	staged, err := outbox.Published{}.Staged(world.Fields(world.Stored(key)), events.Types{}.EmailReceived())
	Assert.IsTrue(first == nil && second == nil, "accept failed")
	Assert.IsTrue(err == nil, "pending events unreadable")
	Assert.Equals(1, len(staged))
	Assert.Equals(1, len(world.Pusher.Attempted()))
}

func (test *OutboxTest) TestAcceptPushesTheEventToMessagingAtOnce() {
	world := InboundTestSetup{}.World()
	key := world.FreshKey()
	accepted, err := world.Inbound.Accept(world.Envelope(key))
	world.Must(err)
	pushed := world.PushedToMessaging()
	payload := events.EmailReceived{}
	Assert.Equals(1, len(pushed))
	world.Must(events.PayloadReader{}.Read(pushed[0], &payload))
	Assert.Equals(accepted.InboundId, payload.InboundId)
	Assert.Equals(0, world.Stored(key).OutboxPending)
}

func (test *OutboxTest) TestARefusedEventBecomesADeadLetterThatTheRelayDeliversAfterRequeue() {
	world := InboundTestSetup{}.World()
	world.Pusher.FailFor("messaging", outbox.DeliveryFailure{Subscriber: "messaging", Status: 400, Permanent: true, Detail: "bad"})
	key := world.FreshKey()
	_, err := world.Inbound.Accept(world.Envelope(key))
	world.Must(err)
	stored := world.Stored(key)
	dead := test.deadFor(world, stored.ID)
	Assert.Equals(1, len(dead))
	Assert.Equals("messaging", dead[0].Subscriber)
	Assert.Equals("email.received", dead[0].Type)
	Assert.Equals(world.Collection(), dead[0].Collection)
	world.Pusher.Recover("messaging")
	controller := &OutboxController{relay: world.Relay}
	requeued := controller.PostRequeue(RequeueRequest{Collection: dead[0].Collection, DocumentId: dead[0].DocumentId, EventId: dead[0].EventId, Subscriber: "messaging"})
	_, relayed := controller.PostRelay()
	Assert.IsTrue(requeued == nil, "requeue failed")
	Assert.IsTrue(relayed == nil, "relay failed")
	Assert.Equals(1, len(world.PushedToMessaging()))
	Assert.Equals(0, world.Stored(key).OutboxPending)
	Assert.Equals(0, len(test.deadFor(world, stored.ID)))
}

func (test *OutboxTest) TestRequeueOfAnUnknownCollectionIsNotFound() {
	world := InboundTestSetup{}.World()
	err := world.Relay.Requeue(RequeueRequest{Collection: "inbound-raw-" + world.RandomText(), DocumentId: "1", EventId: "evt_1", Subscriber: "messaging"})
	Assert.IsTrue(world.IsNotFound(err), "want NotFound")
}

func (test *OutboxTest) TestRequeueOfAnUnknownLetterIsNotFound() {
	world := InboundTestSetup{}.World()
	key := world.FreshKey()
	_, err := world.Inbound.Accept(world.Envelope(key))
	world.Must(err)
	missing := world.Relay.Requeue(RequeueRequest{Collection: world.Collection(), DocumentId: world.Stored(key).ID, EventId: "evt_missing", Subscriber: "messaging"})
	Assert.IsTrue(world.IsNotFound(missing), "want NotFound")
}

func (test *OutboxTest) TestLoadingAMissingDocumentIsNotFound() {
	world := InboundTestSetup{}.World()
	_, err := world.Raw.Collection(world.Domain)
	world.Must(err)
	_, missing := world.Store.Load(outbox.Ref{Collection: world.Collection(), Id: "00000000000000"})
	Assert.IsTrue(world.IsNotFound(missing), "want NotFound")
}

func (test *OutboxTest) TestTheSweepSurvivesARegisteredCollectionThatIsMissing() {
	world := InboundTestSetup{}.World()
	world.Must(world.Registry.Register("inbound-raw-missing-" + world.RandomText()))
	_, err := (&OutboxController{relay: world.Relay}).PostRelay()
	Assert.IsTrue(err == nil, "the sweep failed")
}

func (test *OutboxTest) TestTheRegistryRefusesAnUnknownCollection() {
	world := InboundTestSetup{}.World()
	Assert.IsTrue(world.IsNotFound(world.Registry.Check("not-a-collection")), "want NotFound")
}

func (test *OutboxTest) TestTheRegistryAcceptsARegisteredCollection() {
	world := InboundTestSetup{}.World()
	_, err := world.Raw.Collection(world.Domain)
	world.Must(err)
	Assert.IsTrue(world.Registry.Check(world.Collection()) == nil, "want no error")
}

func (test *OutboxTest) deadFor(world *InboundTestSetup, documentId string) []DeadEvent {
	dead, err := (&OutboxController{relay: world.Relay}).GetDead()
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
