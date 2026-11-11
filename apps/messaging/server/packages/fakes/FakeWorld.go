package fakes

import (
	"crypto/rand"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/outbox"
)

type FakeWorld struct {
	Stores    core.Stores
	Ports     core.Ports
	Directory *FakeDirectory
	Renderer  *FakeRenderer
	Inbound   *FakeInbound
	Clock     *Clock
}

func NewFakeWorld() *FakeWorld {
	world := &FakeWorld{Stores: core.MemoryStores{}.New(), Directory: NewFakeDirectory(), Renderer: &FakeRenderer{}, Inbound: NewFakeInbound()}
	world.Clock = &Clock{At: time.Date(2026, time.November, 10, 12, 0, 0, 0, time.UTC)}
	stager := core.NewStager(world.OutboxSettings(), world.Clock.Now, rand.Reader)
	world.Ports = core.Ports{Directory: world.Directory, Renderer: world.Renderer, Inbound: world.Inbound, Stager: stager, Now: world.Clock.Now}
	return world
}

func (world *FakeWorld) OutboxSettings() outbox.Settings {
	return outbox.Settings{Producer: "messaging", Subscribers: []outbox.Subscriber{
		{Name: "realtime", URL: "http://127.0.0.1:1/realtime"},
		{Name: "notify", URL: "http://127.0.0.1:1/notify"},
		{Name: "search", URL: "http://127.0.0.1:1/search"},
		{Name: "mail-out", URL: "http://127.0.0.1:1/mail-out"},
	}}
}
