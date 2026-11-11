package attachments

import (
	"net/http"
	"time"

	"mesail.dev/packages/oidc"
)

type DeskFactory struct {
	Environment func(string) string
}

func (factory DeskFactory) Desk(documents Documents) Desk {
	settings := SettingsReader{Environment: factory.Environment}.Read()
	store, err := StoreFactory{Environment: factory.Environment}.Store(settings)
	readers := MessagingAccess{BaseUrl: settings.Messaging, Client: http.DefaultClient}
	return Desk{
		Gate: oidc.Configured(), Book: RecordBook{Documents: documents, Now: time.Now}, Store: store,
		Access: AccessRules{Readers: readers}, Settings: settings, Now: time.Now, Failure: err,
	}
}
