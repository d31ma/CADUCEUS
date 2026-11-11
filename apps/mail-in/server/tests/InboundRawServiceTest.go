import (
	"crypto/rand"
	"strings"
)

// @Test(InboundTestSetup, {"partition":"tests"})
type InboundRawServiceTest struct {
	behaviors *BehaviorContext
}

func (test *InboundRawServiceTest) collection() string {
	return "inbound-raw-test-" + strings.ToLower(rand.Text())
}

func (test *InboundRawServiceTest) TestInsertGivesTheDocumentAnId() {
	service := &InboundRawService{}
	collection := test.collection()
	err := service.EnsureCollection(collection)
	stored, inserted := service.Insert(collection, InboundRaw{DomainId: "example.com", RawBlobKey: "example.com/inbound/a.eml", Rcpts: []string{"ana@example.com"}})
	Assert.IsTrue(err == nil, "collection not created")
	Assert.IsTrue(inserted == nil, "insert failed")
	Assert.IsTrue(stored.ID != "", "no id")
}

func (test *InboundRawServiceTest) TestFindByBlobKeyFindsTheInsertedDocument() {
	service := &InboundRawService{}
	collection := test.collection()
	service.EnsureCollection(collection)
	stored, _ := service.Insert(collection, InboundRaw{DomainId: "example.com", RawBlobKey: "example.com/inbound/b.eml", Rcpts: []string{"ana@example.com"}})
	found, err := service.FindByBlobKey(collection, "example.com/inbound/b.eml")
	Assert.IsTrue(err == nil, "find failed")
	Assert.Equals(stored.ID, found[0].ID)
}

func (test *InboundRawServiceTest) TestAddEventMarksTheEventQueued() {
	service := &InboundRawService{}
	collection := test.collection()
	service.EnsureCollection(collection)
	stored, _ := service.Insert(collection, InboundRaw{DomainId: "example.com", RawBlobKey: "example.com/inbound/c.eml", Rcpts: []string{"ana@example.com"}})
	err := service.AddEvent(collection, stored.ID, InboundEventPatch{PendingEvents: []string{"{}"}, EventQueued: true})
	found, _ := service.FindByBlobKey(collection, "example.com/inbound/c.eml")
	Assert.IsTrue(err == nil, "patch failed")
	Assert.IsTrue(found[0].EventQueued, "event not marked")
}

func (test *InboundRawServiceTest) TestEnsureCollectionIsSafeToRepeat() {
	service := &InboundRawService{}
	collection := test.collection()
	first := service.EnsureCollection(collection)
	second := service.EnsureCollection(collection)
	Assert.IsTrue(first == nil && second == nil, "repeat failed")
}
