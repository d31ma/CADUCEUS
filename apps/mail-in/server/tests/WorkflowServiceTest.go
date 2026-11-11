import (
	"os"

	"mesail.dev/packages/mailin"
)

// @Test(InboundTestSetup, {"partition":"tests"})
type WorkflowServiceTest struct {
	behaviors *BehaviorContext
}

func (test *WorkflowServiceTest) TestInboxRegistersTheCollectionForTheOutbox() {
	world := InboundTestSetup{}.World()
	collection, err := world.Workflows.Inbox().Collection(world.Domain)
	Assert.IsTrue(err == nil, "collection not created")
	Assert.Equals(world.Collection(), collection)
	Assert.IsTrue(world.Workflows.Inbox().Registry.Check(collection) == nil, "collection not registered")
}

func (test *WorkflowServiceTest) TestOutboxSweepsOneHundredDocuments() {
	world := InboundTestSetup{}.World()
	Assert.Equals(100, world.Workflows.Outbox().SweepSize())
}

func (test *WorkflowServiceTest) TestSettingsComeFromTheApplicationJson() {
	InboundTestSetup{}.World()
	settings, err := mailin.SettingsReader{Environment: os.Getenv}.Outbox()
	Assert.IsTrue(err == nil, "settings missing")
	Assert.Equals("mail-in", settings.Producer)
	Assert.Equals("messaging", settings.Subscribers[0].Name)
	Assert.Equals(12, settings.Retry.MaxAttempts)
	Assert.Equals("http://127.0.0.1:8104/api/v1/messaging/events", settings.Subscribers[0].URL)
}
