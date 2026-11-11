import (
	"errors"
	"slices"
)

// @Test(NotifyTestSetup, {"partition":"tests"})
type OutboxCollectionsServiceTest struct {
	behaviors *BehaviorContext
}

func (test *OutboxCollectionsServiceTest) TestRegisterThenCheck() {
	collection := NotifyTestSetup{}.Name("notify-summaries")
	registry := NotifyTestSetup{}.Registry()
	err := registry.Register(collection)
	Assert.IsTrue(err == nil, "register failed")
	Assert.IsTrue(registry.Check(collection) == nil, "check failed")
}

func (test *OutboxCollectionsServiceTest) TestCheckRefusesAnUnknownCollection() {
	err := NotifyTestSetup{}.Registry().Check(NotifyTestSetup{}.Name("unknown"))
	missing := &NotFound{}
	Assert.IsTrue(errors.As(err, &missing))
}

func (test *OutboxCollectionsServiceTest) TestAllListsTheRegisteredCollection() {
	collection := NotifyTestSetup{}.Name("notify-summaries")
	registry := NotifyTestSetup{}.Registry()
	registry.Register(collection)
	names, err := registry.All(100000)
	Assert.IsTrue(err == nil, "list failed")
	Assert.IsTrue(slices.Contains(names, collection), "collection missing")
}
