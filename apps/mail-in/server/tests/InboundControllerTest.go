import (
	"errors"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Test(InboundTestSetup, {"partition":"tests"})
type InboundControllerTest struct {
	behaviors *BehaviorContext
}

func (test *InboundControllerTest) TestPostMessageRefusesAnEnvelopeWithoutRecipients() {
	world := InboundTestSetup{}.World()
	_, err := world.Inbound.PostMessage(InboundEnvelope{RawBlobKey: world.FreshKey(), Size: 1})
	Assert.IsTrue(world.IsInvalid(err), "want InvalidEnvelope")
}

func (test *InboundControllerTest) TestPostMessageRefusesAKeyOfAnotherDomain() {
	world := InboundTestSetup{}.World()
	_, err := world.Inbound.PostMessage(world.Envelope("example.org/inbound-x"))
	Assert.IsTrue(world.IsInvalid(err), "want InvalidEnvelope")
}

func (test *InboundControllerTest) TestPostMessageStoresTheMessageOnce() {
	world := InboundTestSetup{}.World()
	key := world.FreshKey()
	first, err := world.Inbound.PostMessage(world.Envelope(key))
	second, again := world.Inbound.PostMessage(world.Envelope(key))
	stored := world.Stored(key)
	Assert.IsTrue(err == nil, "first accept failed")
	Assert.IsTrue(again == nil, "second accept failed")
	Assert.Equals(first.InboundId, second.InboundId)
	Assert.Equals("in_"+stored.ID, first.InboundId)
	Assert.Equals("accepted", stored.State)
	Assert.Equals("ana@"+world.Domain, stored.Recipients[0])
}

func (test *InboundControllerTest) TestPostMessageStagesOneEmailReceivedEvent() {
	world := InboundTestSetup{}.World()
	world.Pusher.FailFor("messaging", errors.New("connection refused"))
	key := world.FreshKey()
	accepted, err := world.Inbound.PostMessage(world.Envelope(key))
	world.Must(err)
	stored := world.Stored(key)
	payload := events.EmailReceived{}
	envelope, staged := outbox.Published{}.One(stored.OutboxFields(), events.Types{}.EmailReceived(), &payload)
	Assert.IsTrue(staged == nil, "no single staged event")
	Assert.Equals(world.Domain, envelope.DomainId)
	Assert.Equals("mail-in", envelope.Producer)
	Assert.Equals(accepted.InboundId, payload.InboundId)
	Assert.Equals("ana@"+world.Domain, payload.Rcpts[0])
	Assert.Equals(key, payload.RawBlobKey)
	Assert.Equals("dan@outside.test", payload.Headers.From)
	Assert.Equals("ana@"+world.Domain, payload.Headers.To[0])
	Assert.Equals("Hello", payload.Headers.Subject)
	Assert.IsTrue(payload.Headers.Cc != nil && len(payload.Headers.Cc) == 0, "cc must be an empty list")
	Assert.Equals("pass", payload.AuthResults.Dmarc)
	Assert.Equals(1, stored.OutboxPending)
	Assert.IsTrue(stored.EventQueued, "event not marked")
}

func (test *InboundControllerTest) TestARetriedMessageStagesNoSecondEvent() {
	world := InboundTestSetup{}.World()
	world.Pusher.FailFor("messaging", errors.New("connection refused"))
	key := world.FreshKey()
	_, first := world.Inbound.PostMessage(world.Envelope(key))
	_, second := world.Inbound.PostMessage(world.Envelope(key))
	staged, err := outbox.Published{}.Staged(world.Stored(key).OutboxFields(), events.Types{}.EmailReceived())
	Assert.IsTrue(first == nil && second == nil, "accept failed")
	Assert.IsTrue(err == nil, "pending events unreadable")
	Assert.Equals(1, len(staged))
	Assert.Equals(1, len(world.Pusher.Attempted()))
}

func (test *InboundControllerTest) TestPostMessagePushesTheEventToMessagingAtOnce() {
	world := InboundTestSetup{}.World()
	key := world.FreshKey()
	accepted, err := world.Inbound.PostMessage(world.Envelope(key))
	world.Must(err)
	pushed := world.PushedToMessaging()
	payload := events.EmailReceived{}
	Assert.Equals(1, len(pushed))
	world.Must(events.PayloadReader{}.Read(pushed[0], &payload))
	Assert.Equals(accepted.InboundId, payload.InboundId)
	Assert.Equals(0, world.Stored(key).OutboxPending)
}
