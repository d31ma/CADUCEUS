import "errors"

// @Test(InboundTestSetup, {"partition":"tests"})
type InboundServiceTest struct {
	behaviors *BehaviorContext
}

func (test *InboundServiceTest) TestAcceptRefusesAnInvalidEnvelope() {
	world := InboundTestSetup{}.World()
	_, err := world.Inbound.Accept(world.Envelope("example.org/inbound-x"))
	invalid := &InvalidEnvelope{}
	Assert.IsTrue(errors.As(err, &invalid))
}

func (test *InboundServiceTest) TestAcceptStoresTheMessageOnce() {
	world := InboundTestSetup{}.World()
	key := world.FreshKey()
	first, err := world.Inbound.Accept(world.Envelope(key))
	second, again := world.Inbound.Accept(world.Envelope(key))
	Assert.IsTrue(err == nil, "first accept failed")
	Assert.IsTrue(again == nil, "second accept failed")
	Assert.Equals(first.InboundId, second.InboundId)
}
