package channels

import (
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/transport"
)

type Assembly struct {
	Source    Commander
	Pusher    outbox.Pusher
	Directory Directory
}

func (assembly Assembly) Workflow() Workflow {
	settings := SettingsSource{}.FromEnvironment()
	ports := Ports{
		Documents: NewCommandDocuments(assembly.Source),
		Relay:     assembly.desk(settings),
		Directory: assembly.directory(settings),
		Callers:   TokenCallers{},
		Settings:  settings,
		Now:       time.Now,
	}
	return NewWorkflow(ports)
}

func (assembly Assembly) Desk() OutboxDesk {
	return assembly.desk(SettingsSource{}.FromEnvironment())
}

func (assembly Assembly) desk(settings Settings) OutboxDesk {
	return NewOutboxDesk(NewCommandDocuments(assembly.Source), assembly.pusher(), settings, time.Now)
}

func (assembly Assembly) pusher() outbox.Pusher {
	if assembly.Pusher != nil {
		return assembly.Pusher
	}
	return RealmPusher{Tokens: oidc.ServiceTokensFactory{}.Configured(), Timeout: outbox.HTTPPusher{}.DefaultTimeout()}
}

func (assembly Assembly) directory(settings Settings) Directory {
	if assembly.Directory != nil {
		return assembly.Directory
	}
	return NewDirectoryClient(settings.DirectoryUrl, transport.SingleUseTransport{}.Client(0))
}
