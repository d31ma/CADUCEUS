import "slices"

// @Test(InboundTestSetup, {"partition":"tests"})
type InboundRawServiceTest struct {
	behaviors *BehaviorContext
}

func (test *InboundRawServiceTest) TestCollectionIsRegisteredForTheOutbox() {
	world := InboundTestSetup{}.World()
	collection, err := world.Raw.Collection(world.Domain)
	known, listed := world.Registry.All()
	Assert.IsTrue(err == nil, "collection not created")
	Assert.IsTrue(listed == nil, "registry not read")
	Assert.Equals(world.Collection(), collection)
	Assert.IsTrue(slices.Contains(known, collection), "collection not registered")
}

func (test *InboundRawServiceTest) TestCollectionIsSafeToRepeat() {
	world := InboundTestSetup{}.World()
	_, first := world.Raw.Collection(world.Domain)
	_, second := world.Raw.Collection(world.Domain)
	Assert.IsTrue(first == nil && second == nil, "repeat failed")
}

func (test *InboundRawServiceTest) TestInsertGivesTheDocumentAnId() {
	world := InboundTestSetup{}.World()
	collection, _ := world.Raw.Collection(world.Domain)
	stored, err := world.Raw.Insert(collection, InboundRaw{DomainId: world.Domain, RawBlobKey: world.FreshKey(), Rcpts: []string{"ana@" + world.Domain}})
	Assert.IsTrue(err == nil, "insert failed")
	Assert.IsTrue(stored.ID != "", "no id")
}

func (test *InboundRawServiceTest) TestFindByBlobKeyFindsTheInsertedDocument() {
	world := InboundTestSetup{}.World()
	collection, _ := world.Raw.Collection(world.Domain)
	key := world.FreshKey()
	stored, _ := world.Raw.Insert(collection, InboundRaw{DomainId: world.Domain, RawBlobKey: key, Rcpts: []string{"ana@" + world.Domain}})
	found, err := world.Raw.FindByBlobKey(collection, key)
	Assert.IsTrue(err == nil, "find failed")
	Assert.Equals(1, len(found))
	Assert.Equals(stored.ID, found[0].ID)
}

func (test *InboundRawServiceTest) TestSaveEventMarksTheEventQueued() {
	world := InboundTestSetup{}.World()
	collection, _ := world.Raw.Collection(world.Domain)
	key := world.FreshKey()
	stored, _ := world.Raw.Insert(collection, InboundRaw{DomainId: world.Domain, RawBlobKey: key, Rcpts: []string{"ana@" + world.Domain}})
	err := world.Raw.SaveEvent(collection, stored.ID, InboundEventPatch{PendingEvents: []string{}, DeadEvents: []string{}, EventQueued: true})
	Assert.IsTrue(err == nil, "patch failed")
	Assert.IsTrue(world.Stored(key).EventQueued, "event not marked")
}
