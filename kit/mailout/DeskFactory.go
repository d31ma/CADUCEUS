package mailout

import (
	"errors"
	"net"
	"strconv"
	"time"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/dkim"
	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
	"mesail.dev/packages/smtpclient"
	"mesail.dev/packages/transport"
)

type DeskFactory struct {
	Environment func(string) string
}

func (DeskFactory) variable() string { return "YON_APPLICATION_JSON" }

func (factory DeskFactory) Desk(documents Documents) Desk {
	settings := SettingsReader{Environment: factory.Environment}.Read()
	outboxSettings, err := outbox.SettingsReader{}.FromApplicationJSON(factory.Environment(factory.variable()))
	dialer, dialErr := smtpclient.ProxyDialers{}.FromEnvironment()
	authorize := factory.tokens(settings).Authorization
	store := DocumentStore{Documents: documents}
	registry := Registry{Store: store}
	client := ServiceClient{Client: transport.SingleUseTransport{}.Client(settings.Timeout()), Authorize: authorize}
	directory := Directory{Client: client, BaseUrl: settings.DirectoryUrl}
	services := events.Services{}
	sources := Sources{Client: client, Bases: map[string]string{services.Messaging(): settings.MessagingUrl, services.Notify(): settings.NotifyUrl}}
	queue := Queue{Store: store, Registry: registry, Outbox: outboxSettings, Lease: settings.Lease()}
	relayer := Relayer{Store: OutboxStore{Store: store, Registry: registry}, Settings: outboxSettings, Client: client, Now: time.Now}
	changes := ChangeEvents{Now: time.Now}
	builder := Builder{Directory: directory, Sources: sources, Blobs: factory.blobs(settings), Keys: dkim.KeyStore{Source: factory.secrets(settings)}}
	transport := Transport{Directory: directory, MailIn: MailIn{Client: client, BaseUrl: settings.MailInUrl}, Relay: factory.relay(settings, dialer), Direct: factory.direct(settings), DirectEnabled: settings.DirectEnabled, Timeout: settings.Timeout()}
	suppressions := Suppressions{Store: store}
	outcomes := Outcomes{Queue: queue, Suppressions: suppressions, Attempts: AttemptLog{Store: store}, Relayer: relayer, Backoff: Backoff{First: settings.FirstDelay(), Largest: settings.MaxDelay()}, Changes: changes}
	delivery := Delivery{Queue: queue, Suppressions: suppressions, Builder: builder, Transport: transport, Outcomes: outcomes}
	intake := Intake{Queue: queue, Relayer: relayer, Sources: sources, Settings: settings, Changes: changes, Now: time.Now}
	return Desk{
		Gate: oidc.Configured(), Seen: SeenEvents{Store: store, Now: time.Now}, Intake: intake,
		Run: QueueRun{Registry: registry, Queue: queue, Delivery: delivery, Batch: 50}, Relayer: relayer, Now: time.Now, Limit: 100,
		Failure: errors.Join(err, dialErr),
	}
}

func (factory DeskFactory) tokens(settings Settings) oidc.ServiceTokens {
	tokens := oidc.ServiceTokensFactory{}.FromJson(factory.Environment(factory.variable()))
	tokens.Client = transport.SingleUseTransport{}.Client(settings.Timeout())
	return tokens
}

func (DeskFactory) blobs(settings Settings) blobs.Store {
	if settings.BlobFolder == "" {
		return MissingBlobs{}
	}
	return blobs.NewLocalFolder(settings.BlobFolder, "", blobs.NewLocalSigner(nil))
}

func (DeskFactory) secrets(settings Settings) dkim.SecretSource {
	if settings.SecretFolder != "" {
		return dkim.FolderSecretSource{Root: settings.SecretFolder}
	}
	return dkim.EnvironmentSecretSource{Prefix: settings.SecretPrefix}
}

func (DeskFactory) relay(settings Settings, dialer smtpclient.Dialer) smtpclient.Delivery {
	return smtpclient.RelayDelivery{Address: settings.RelayAddress, Dialer: dialer, HeloName: settings.HeloName, Username: settings.RelayUsername, Password: settings.RelayPassword, RequireTls: settings.RelayRequireTls, ImplicitTls: settings.RelayImplicitTls}
}

func (DeskFactory) direct(settings Settings) smtpclient.Delivery {
	dialer := smtpclient.DirectDialer{Timeout: settings.Timeout()}
	if settings.DirectMxHost != "" {
		return smtpclient.RelayDelivery{Address: net.JoinHostPort(settings.DirectMxHost, strconv.Itoa(settings.DirectPort)), Dialer: dialer, HeloName: settings.HeloName}
	}
	return smtpclient.DirectDelivery{Hosts: smtpclient.MxHosts{Resolver: net.DefaultResolver}, Dialer: dialer, HeloName: settings.HeloName, Port: settings.DirectPort}
}
