package attachments

import (
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/transport"
)

type DeskFactory struct {
	Environment func(string) string
}

func (factory DeskFactory) Desk(documents Documents) Desk {
	settings := SettingsReader{Environment: factory.Environment}.Read()
	store, err := StoreFactory{Environment: factory.Environment}.Store(settings)
	readers := MessagingAccess{BaseUrl: settings.Messaging, Client: transport.SingleUseTransport{}.Client(0)}
	return Desk{
		Gate: oidc.Configured(), Book: RecordBook{Documents: documents, Now: time.Now}, Store: store,
		Access: AccessRules{Readers: readers}, Settings: settings, Now: time.Now, Failure: err,
	}
}
