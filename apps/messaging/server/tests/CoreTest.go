import (
	"mesail.dev/packages/adapters"
	"mesail.dev/packages/core"
	"mesail.dev/packages/fakes"
)

// @Test(StorageTestSetup, {"partition":"tests"})
type CoreTest struct {
	behaviors *BehaviorContext
}

func (test *CoreTest) TestBuiltStoresWriteToFylo() {
	world := StorageTestSetup{}.World()
	stores := world.Core().Stores()
	id, err := stores.Conversations.Insert(core.Conversation{DomainId: world.Domain, Kind: "direct"})
	world.Must(err)
	_, found, _ := world.Conversations().Find(world.Domain, id)
	Assert.IsTrue(found, "the built store writes to FYLO")
	_, err = stores.AttachmentUses.Insert(core.AttachmentUse{DomainId: world.Domain, AttachmentId: "att_1", MessageId: "msg_1"})
	Assert.IsTrue(err == nil, "the built stores hold the attachment uses")
}

func (test *CoreTest) TestBuiltPortsUseTheSettings() {
	world := StorageTestSetup{}.World()
	ports := world.Core().Ports()
	_, isPlain := ports.Renderer.(adapters.PlainRenderer)
	_, isDirectory := ports.Directory.(adapters.DirectoryClient)
	_, isMailIn := ports.Inbound.(adapters.MailInClient)
	Assert.IsTrue(isPlain && isDirectory && isMailIn, "the built ports are the HTTP adapters")
	Assert.IsTrue(!ports.Now().IsZero(), "the clock runs")
	fields, err := ports.Stager.Stage(core.OutboxFields{}, "message.deleted", world.Domain, map[string]string{})
	Assert.IsTrue(err == nil, "staging works")
	Assert.Equals(1, fields.OutboxPending)
	_, err = ports.Directory.Resolve("Bearer caller", []string{"a@example.com"})
	Assert.IsTrue(world.IsUnavailable(err), "the configured directory is not running")
}

func (test *CoreTest) TestFixedStoresAndPortsWin() {
	fake := fakes.NewFakeWorld()
	core := &CoreService{fixedStores: &fake.Stores, fixedPorts: &fake.Ports}
	Assert.IsTrue(core.Stores().Conversations == fake.Stores.Conversations, "fixed stores")
	Assert.IsTrue(core.Ports().Directory == fake.Ports.Directory, "fixed ports")
}
