package mailin

import (
	"crypto/rand"
	"time"

	"mesail.dev/packages/outbox"
)

type DeskFactory struct {
	Environment func(string) string
	Pusher      outbox.Pusher
}

func (factory DeskFactory) Inbox(documents Documents, pages PageReader) Inbox {
	storage := Storage{Documents: documents, Pages: pages}
	stager := EventStager{Settings: factory.settings(), Now: time.Now, Random: rand.Reader}
	return Inbox{Storage: storage, Registry: CollectionRegistry{Storage: storage}, Stager: stager, Outbox: factory.Outbox(documents, pages), Now: time.Now}
}

func (factory DeskFactory) Outbox(documents Documents, pages PageReader) OutboxDesk {
	storage := Storage{Documents: documents, Pages: pages}
	store := OutboxStore{Storage: storage, Registry: CollectionRegistry{Storage: storage}}
	return OutboxDesk{Store: store, Pusher: factory.pusher(), Settings: factory.settings(), Now: time.Now}
}

func (factory DeskFactory) settings() SettingsReader {
	return SettingsReader{Environment: factory.Environment}
}

func (factory DeskFactory) pusher() outbox.Pusher {
	if factory.Pusher != nil {
		return factory.Pusher
	}
	return outbox.NewHTTPPusher(factory.settings().ServiceToken(), outbox.HTTPPusher{}.DefaultTimeout())
}
