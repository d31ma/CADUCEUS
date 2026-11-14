package realtime

import (
	"crypto/rand"
	"os"
	"time"

	"mesail.dev/packages/oidc"
	"mesail.dev/packages/transport"
)

type DeskFactory struct {
	Environment func(string) string
}

func (factory DeskFactory) Gateway(storage Storage) Gateway {
	settings := factory.settings()
	parts := factory.parts(storage, settings)
	directory := factory.directory(settings)
	sessions := Sessions{Tokens: factory.tokens(), Tickets: factory.ticketBook(storage), Connections: parts.Connections, Domains: DomainBook{Storage: storage}, Presence: parts.Presence, Devices: DeviceLinks{Directory: directory}}
	typing := Typing{Members: MemberBook{Storage: storage}, FanOut: parts.FanOut}
	return Gateway{Sessions: sessions, Typing: typing, Presence: parts.Presence, Connections: parts.Connections, Pushes: factory.pushes(settings), Signalling: factory.signalling(storage, settings, parts, directory)}
}

func (factory DeskFactory) CallSigner() CallSigner {
	return CallSignerFactory{}.FromSettings(factory.settings().CallKeys, time.Now)
}

func (factory DeskFactory) Tickets(storage Storage) Tickets {
	return Tickets{Tokens: factory.tokens(), Book: factory.ticketBook(storage), Now: time.Now}
}

func (factory DeskFactory) Presence(storage Storage) PresenceDesk {
	parts := factory.parts(storage, factory.settings())
	return PresenceDesk{Gate: oidc.Configured(), Presence: parts.Presence}
}

func (factory DeskFactory) Events(storage Storage) EventDesk {
	parts := factory.parts(storage, factory.settings())
	return EventDesk{Gate: oidc.Configured(), Seen: SeenBook{Storage: storage}, Members: MemberBook{Storage: storage}, FanOut: parts.FanOut}
}

func (factory DeskFactory) Sweeper(storage Storage) Sweeper {
	settings := factory.settings()
	parts := factory.parts(storage, settings)
	domains := DomainSweep{Book: DomainBook{Storage: storage}, Connections: parts.Connections, Pushes: factory.pushes(settings), Presence: parts.Presence, Calls: CallSweep{Calls: CallBook{Storage: storage}, Seats: SeatBook{Storage: storage}}}
	return Sweeper{Gate: oidc.Configured(), Domains: domains, Tickets: TicketSweep{Book: factory.ticketBook(storage)}}
}

func (factory DeskFactory) GatewayCheck() GatewayCheck {
	return GatewayCheck{Secret: factory.settings().GatewaySecret}
}

func (factory DeskFactory) parts(storage Storage, settings Settings) DeskParts {
	connections := ConnectionBook{Storage: storage, Now: time.Now}
	fanOut := FanOut{Connections: connections, Gateway: factory.pushes(settings)}
	presence := PresenceKeeper{Book: PresenceBook{Storage: storage}, Connections: connections, FanOut: fanOut, Now: time.Now}
	return DeskParts{Connections: connections, FanOut: fanOut, Presence: presence}
}

func (factory DeskFactory) signalling(storage Storage, settings Settings, parts DeskParts, directory DeviceDirectory) Signalling {
	pushes := factory.pushes(settings)
	pairings := PairingBook{Storage: storage, Now: time.Now}
	signals := Signals{Connections: parts.Connections, Directory: directory, Pairings: pairings, Pushes: pushes}
	calls := Calls{Book: CallBook{Storage: storage}, Seats: SeatBook{Storage: storage}, Starts: CallStartBook{Storage: storage, Now: time.Now}, Signer: CallSignerFactory{}.FromSettings(settings.CallKeys, time.Now), Connections: parts.Connections, Pushes: pushes, Notify: NotifyClient{Call: factory.serviceCall(settings.NotifyUrl, SignalLimits{}.CallPushTimeout())}, Log: StderrCallPushLog{Output: os.Stderr}, Now: time.Now}
	return Signalling{Signals: signals, Pairings: Pairings{Book: pairings, Connections: parts.Connections, Pushes: pushes}, Calls: calls, Pushes: pushes}
}

func (factory DeskFactory) directory(settings Settings) DeviceDirectory {
	return DirectoryClient{Call: factory.serviceCall(settings.DirectoryUrl, factory.pushTimeout())}
}

func (factory DeskFactory) serviceCall(baseUrl string, timeout time.Duration) ServiceCall {
	return ServiceCall{BaseUrl: baseUrl, Tokens: oidc.ServiceTokensFactory{}.Configured(), Client: transport.SingleUseTransport{}.Client(timeout)}
}

func (factory DeskFactory) settings() Settings {
	return SettingsReader{Environment: factory.Environment}.Read()
}

func (DeskFactory) tokens() TokenChecker {
	return OidcTokenChecker{Verifier: oidc.VerifierFactory{}.FromEnvironment()}
}

func (DeskFactory) ticketBook(storage Storage) TicketBook {
	return TicketBook{Storage: storage, Now: time.Now, Random: rand.Reader}
}

func (factory DeskFactory) pushes(settings Settings) ConnectionGateway {
	return HttpConnectionGateway{BaseUrl: settings.GatewayUrl, Secret: settings.GatewaySecret, Client: transport.SingleUseTransport{}.Client(factory.pushTimeout())}
}

func (DeskFactory) pushTimeout() time.Duration {
	return 5 * time.Second
}
