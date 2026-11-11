package mailin

import (
	"crypto/rand"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/transport"
)

type DeskFactory struct {
	Environment func(string) string
	Pusher      outbox.Pusher
}

func (factory DeskFactory) Inbox(documents Documents, pages PageReader) Inbox {
	storage := Storage{Documents: documents, Pages: pages}
	stager := EventStager{Settings: factory.settings(), Now: time.Now, Random: rand.Reader}
	return Inbox{Storage: storage, Registry: CollectionRegistry{Storage: storage}, Stager: stager, Outbox: factory.Outbox(documents, pages), HandOff: factory.handOff(), Now: time.Now}
}

func (factory DeskFactory) Outbox(documents Documents, pages PageReader) OutboxDesk {
	storage := Storage{Documents: documents, Pages: pages}
	store := OutboxStore{Storage: storage, Registry: CollectionRegistry{Storage: storage}}
	return OutboxDesk{Store: store, Pusher: factory.pusher(), Settings: factory.settings(), Now: time.Now}
}

func (factory DeskFactory) Messages(documents Documents, pages PageReader) MessageDesk {
	storage := Storage{Documents: documents, Pages: pages}
	settings := factory.settings().MailIn()
	store := factory.blobs()
	contents := ContentShelf{Storage: storage, Builder: ContentBuilder{Blobs: store}}
	linkLife := time.Duration(settings.Blobs.Link.Seconds) * time.Second
	return MessageDesk{Callers: oidc.Configured(), Book: InboundBook{Storage: storage}, Contents: contents, Blobs: store, LinkLife: linkLife, Now: time.Now}
}

func (DeskFactory) Timeout() time.Duration {
	return 30 * time.Second
}

func (factory DeskFactory) handOff() HandOff {
	directory := Directory{Client: transport.SingleUseTransport{}.Client(factory.Timeout()), BaseUrl: factory.settings().MailIn().Directory, Authorize: factory.settings().Authorize()}
	return HandOff{Domains: directory, Blobs: factory.blobs()}
}

func (factory DeskFactory) blobs() blobs.Store {
	return BlobStoreFactory{Environment: factory.Environment}.Store(factory.settings().MailIn().Blobs)
}

func (factory DeskFactory) settings() SettingsReader {
	return SettingsReader{Environment: factory.Environment}
}

func (factory DeskFactory) pusher() outbox.Pusher {
	if factory.Pusher != nil {
		return factory.Pusher
	}
	tokens := DomainTokens{Authorize: factory.settings().Authorize()}
	return EventPusher{Tokens: tokens, Client: transport.SingleUseTransport{}.Client(outbox.HTTPPusher{}.DefaultTimeout())}
}
