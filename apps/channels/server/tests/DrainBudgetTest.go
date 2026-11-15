import (
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

// @Test(ChannelsTestSetup, {"partition":"tests"})
type DrainBudgetTest struct {
	behaviors *BehaviorContext
}

func (test *DrainBudgetTest) TestCreatingAChannelDoesNotWaitForASlowSubscriberAndTheRelayDeliversLater() {
	world := ChannelsTestSetup{}.World()
	defer world.Close()
	released := make(chan struct{})
	defer close(released)
	slow := test.slowFor(world.Pusher, "realtime", 10*time.Second, released)
	world.Controller = &ChannelsController{channels: &ChannelService{dataSource: DataSource{}, wire: &ChannelWireService{}, failures: &ErrorMappingService{}, pusher: slow, lookup: world.Directory}}
	started := time.Now()
	created := world.Create(world.Member("ana"), "releases", "public")
	took := time.Since(started)
	Assert.IsTrue(took < 5*time.Second, "creating a channel took "+took.String())
	Assert.Equals(1, len(world.Published("messaging", "channel.created", created.ChannelId)))
	Assert.Equals(0, len(world.Published("realtime", "channel.created", created.ChannelId)))
	sweep := test.slowFor(world.Pusher, "realtime", 2500*time.Millisecond, released)
	world.Outbox = &OutboxController{outbox: &OutboxService{dataSource: DataSource{}, failures: &ErrorMappingService{}, pusher: sweep}}
	err := world.RelayUntilDone()
	Assert.IsTrue(err == nil, "relay failed")
	Assert.Equals(1, len(world.Published("realtime", "channel.created", created.ChannelId)))
}

func (test *DrainBudgetTest) slowFor(inner *outbox.RecordingPusher, name string, wait time.Duration, released chan struct{}) outbox.PushFunc {
	return func(subscriber outbox.Subscriber, envelope events.Envelope) error {
		test.waitFor(subscriber.Name == name, wait, released)
		return inner.Push(subscriber, envelope)
	}
}

func (test *DrainBudgetTest) waitFor(slow bool, wait time.Duration, released chan struct{}) {
	if !slow {
		return
	}
	select {
	case <-released:
	case <-time.After(wait):
	}
}
