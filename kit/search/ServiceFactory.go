package search

import (
	"net/http"

	"mesail.dev/packages/oidc"
)

type ServiceFactory struct {
	Environment func(string) string
}

func (factory ServiceFactory) Service(documents Documents) Service {
	settings := SettingsReader{Environment: factory.Environment}.Read()
	reader := NewMembershipClient(settings.MessagingUrl, settings.ChannelsUrl, http.DefaultClient)
	return NewService(documents, reader, oidc.Configured())
}
