package devicebox

import (
	"strings"
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/transport"
)

type Assembly struct {
	Source    Storage
	Pusher    outbox.Pusher
	Directory DeviceDirectory
	Now       func() time.Time
}

func (assembly Assembly) Mailbox() Mailbox {
	parts := assembly.parts()
	return Mailbox{
		callers:   TokenCallers{},
		ownership: NewOwnership(assembly.directory(SettingsSource{}.FromEnvironment())),
		store:     parts.Store,
		counts:    parts.Counts,
		staging:   parts.Staging,
		desk:      parts.Desk,
		now:       assembly.clock(),
	}
}

func (assembly Assembly) Expiry() Expiry {
	return Expiry{callers: TokenCallers{}, parts: assembly.parts(), now: assembly.clock()}
}

func (assembly Assembly) Desk() OutboxDesk {
	return assembly.parts().Desk
}

func (assembly Assembly) parts() ExpiryParts {
	documents := NewDocuments(assembly.Source)
	settings := SettingsSource{}.FromEnvironment()
	return ExpiryParts{
		Store:   NewStore(documents),
		Counts:  NewBoxCounts(documents),
		Staging: NewEventStaging(settings, assembly.clock()),
		Desk:    NewOutboxDesk(documents, assembly.pusher(), settings, assembly.clock()),
	}
}

func (assembly Assembly) pusher() outbox.Pusher {
	if assembly.Pusher != nil {
		return assembly.Pusher
	}
	tokens := oidc.ServiceTokensFactory{}.Configured()
	source := outbox.DomainTokenFunc(func(domainId string) (string, error) {
		authorization, err := tokens.Authorization(domainId)
		return strings.TrimPrefix(authorization, "Bearer "), err
	})
	return outbox.NewDomainHTTPPusher(source, outbox.HTTPPusher{}.DefaultTimeout())
}

func (assembly Assembly) directory(settings Settings) DeviceDirectory {
	if assembly.Directory != nil {
		return assembly.Directory
	}
	return assembly.directoryAt(settings.DirectoryUrl)
}

func (Assembly) directoryAt(directoryUrl string) DeviceDirectory {
	if directoryUrl == "" {
		return UnwiredDirectory{}
	}
	return NewDirectoryClient(directoryUrl, oidc.ServiceTokensFactory{}.Configured(), transport.SingleUseTransport{}.Client(Limits{}.DirectoryTimeout()))
}

func (assembly Assembly) clock() func() time.Time {
	if assembly.Now != nil {
		return assembly.Now
	}
	return time.Now
}
