import (
	"net/http"

	"mesail.dev/packages/outbox"
)

// @Test(DeviceboxTestSetup, {"partition":"tests"})
type OutboxRoutesTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxRoutesTest) TestARelayCallDeliversWhatWaitedAndDeadLettersCanBeRequeued() {
	world := DeviceboxTestSetup{}.World()
	defer world.Close()
	world.Owns("ana", "a1")
	world.Owns("bo", "b1")
	world.Pusher.FailFor("notify", outbox.DeliveryFailure{Subscriber: "notify", Status: http.StatusBadRequest, Permanent: true})
	world.Put("ana", "a1", "b1")
	dead, err := world.Outbox.GetDead()
	Assert.IsTrue(err == nil, "dead letters failed")
	Assert.Equals(1, len(dead))
	world.Pusher.Recover("notify")
	retry := world.Outbox.PostRetry(RetryRequest{Collection: dead[0].Collection, DocumentId: dead[0].DocumentId, EventId: dead[0].EventId, Subscriber: "notify"})
	Assert.IsTrue(retry == nil, "requeue failed")
	err = world.RelayUntilDone()
	Assert.IsTrue(err == nil, "relay failed")
	Assert.Equals(1, len(world.Pusher.PushedTo("notify")))
}

func (test *OutboxRoutesTest) TestRequeueingAnUnknownLetterIsNotFound() {
	world := DeviceboxTestSetup{}.World()
	defer world.Close()
	err := world.Outbox.PostRetry(RetryRequest{Collection: "devicebox-bodies-0000000000000000", DocumentId: "X", EventId: "evt_none", Subscriber: "notify"})
	Assert.Equals(6027, world.NumberOf(err))
}
