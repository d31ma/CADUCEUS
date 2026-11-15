package backups

import (
	"time"

	"mesail.dev/packages/attachments"
	"mesail.dev/packages/oidc"
)

type DeskFactory struct {
	Environment func(string) string
}

func (factory DeskFactory) Desk(documents Documents, scroll Scroller) Desk {
	settings := attachments.SettingsReader{Environment: factory.Environment}.Read()
	store, err := attachments.StoreFactory{}.Store(settings)
	return Desk{
		Gate: oidc.Configured(), Shelf: Shelf{Documents: documents, Scroll: scroll}, Store: store,
		Settings: SettingsReader{Environment: factory.Environment}.Read(), Now: time.Now, Failure: err,
	}
}

func (factory DeskFactory) EventDesk(documents Documents, scroll Scroller) EventDesk {
	seen := SeenEvents{Documents: documents, Scroll: scroll}
	return EventDesk{Gate: oidc.Configured(), Seen: seen, Removal: Removal{Desk: factory.Desk(documents, scroll)}}
}
