import "fmt"

// @Test(DeviceboxTestSetup, {"partition":"tests"})
type WriteCostTest struct {
	behaviors *BehaviorContext
}

func (test *WriteCostTest) TestOnePutForTwoHundredDevicesStaysWithinAFewWrites() {
	world := DeviceboxTestSetup{}.World()
	defer world.Close()
	world.Owns("ana", "a1")
	devices := world.Group(50, 4)
	world.Put("ana", "a1", devices...)
	world.Storage.Reset()
	world.Put("ana", "a1", devices...)
	Assert.IsTrue(world.Storage.Writes() <= 11, fmt.Sprintf("a put for 200 devices took %d writes", world.Storage.Writes()))
	Assert.Equals(2, len(world.Listed("person-49", "p49-d3")))
}

func (test *WriteCostTest) TestListingOneDeviceReadsOnlyItsOwnEnvelopes() {
	world := DeviceboxTestSetup{}.World()
	defer world.Close()
	world.Owns("ana", "a1")
	world.Owns("bo", "b1")
	world.Owns("cy", "c1")
	world.PutMany(25, "ana", "a1", "c1")
	mine := world.Put("ana", "a1", "b1")
	world.Storage.Reset()
	Assert.Equals([]string{mine}, world.Listed("bo", "b1"))
	Assert.Equals(1, world.Storage.Queries())
	Assert.Equals(1, world.Storage.Reads())
	Assert.Equals(0, world.Storage.Writes())
}
