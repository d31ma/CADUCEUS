package storage

import (
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/outbox"
)

type Storage struct {
	Source          DataSource
	Scroll          Scroll
	Pusher          outbox.Pusher
	Settings        outbox.Settings
	SettingsFailure error
}

func (storage Storage) Stores() core.Stores {
	tenant := storage.tenant()
	names := CollectionNames{}
	return core.Stores{
		Conversations:  Conversations{Tenant: tenant, Names: names},
		Participants:   Participants{Tenant: tenant, Names: names},
		Messages:       Messages{Tenant: tenant, Names: names},
		Reactions:      Reactions{Tenant: tenant, Names: names},
		ThreadKeys:     ThreadKeys{Tenant: tenant, Names: names},
		Mentions:       Mentions{Tenant: tenant, Names: names},
		Members:        Members{Tenant: tenant, Names: names},
		AttachmentUses: AttachmentUses{Tenant: tenant, Names: names},
	}
}

func (storage Storage) Seen() SeenEvents {
	return SeenEvents{Tenant: storage.tenant(), Names: CollectionNames{}}
}

func (storage Storage) Outbox() Outbox {
	store := OutboxStore{Documents: storage.documents(), Registry: storage.registry()}
	return Outbox{Store: store, Pusher: storage.Pusher, Settings: storage.Settings, SettingsFailure: storage.SettingsFailure, Now: time.Now}
}

func (storage Storage) Registry() Registry {
	return storage.registry()
}

func (storage Storage) tenant() Tenant {
	return Tenant{Documents: storage.documents(), Registry: storage.registry(), Drain: storage.Outbox().Drain}
}

func (storage Storage) registry() Registry {
	return Registry{Documents: storage.documents(), Names: CollectionNames{}}
}

func (storage Storage) documents() Documents {
	return Documents{Source: storage.Source, Scroll: storage.Scroll}
}
