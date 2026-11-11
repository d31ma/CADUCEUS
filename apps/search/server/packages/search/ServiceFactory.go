package search

import (
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/transport"
)

type ServiceFactory struct {
	Environment func(string) string
}

func (factory ServiceFactory) Service(documents Documents) Service {
	settings := SettingsReader{Environment: factory.Environment}.Read()
	reader := NewMembershipClient(settings.MessagingUrl, settings.ChannelsUrl, transport.SingleUseTransport{}.Client(0))
	return NewService(documents, reader, oidc.Configured())
}
