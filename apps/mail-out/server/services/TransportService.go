import "mesail.dev/packages/smtpclient"

// @Service
type TransportService struct {
	// @Needs
	directory *DirectoryDelegate
	// @Needs
	mailIn *MailInClientDelegate
	// @Needs
	relay *SmtpRelayDelegate
	// @Needs
	direct *SmtpDirectDelegate
	// @Needs
	settings *SettingsDelegate
}

func (TransportService) relayName() string { return "relay" }

func (TransportService) directName() string { return "direct" }

func (TransportService) handOffName() string { return "hand-off" }

func (TransportService) directoryName() string { return "directory" }

func (TransportService) active() string { return "active" }

func (TransportService) installWideRealm() string { return "mesail-operators" }

func (transport *TransportService) Deliver(item OutboundItem, raw []byte, authorize func(string) (string, error)) Outcome {
	domain, found, err := transport.directory.Domain(AddressUtils{}.Domain(item.Rcpt), transport.installWideRealm(), authorize)
	path := transport.path(transport.hosted(domain, found), err == nil)
	return transport.send(item, raw, path, authorize)
}

func (transport *TransportService) hosted(domain Domain, found bool) bool {
	return found && domain.State == transport.active()
}

func (transport *TransportService) path(hosted bool, answered bool) string {
	switch {
	case !answered:
		return transport.directoryName()
	case hosted:
		return transport.handOffName()
	case transport.settings.Settings().DirectEnabled:
		return transport.directName()
	default:
		return transport.relayName()
	}
}

func (transport *TransportService) send(item OutboundItem, raw []byte, path string, authorize func(string) (string, error)) Outcome {
	envelope := smtpclient.Envelope{From: item.From, Recipients: []string{item.Rcpt}}
	switch path {
	case transport.directoryName():
		return Outcome{Status: OutboundStateUtils{}.Deferred(), Transport: path, Detail: "the directory did not answer"}
	case transport.handOffName():
		return OutcomeUtils{}.FromHandOff(transport.mailIn.HandOff(HandOffUtils{}.Envelope(item), AddressUtils{}.Domain(item.Rcpt), authorize))
	case transport.directName():
		reply, err := transport.direct.Deliver(envelope, raw)
		return OutcomeUtils{}.FromSmtp(path, reply, err)
	default:
		reply, err := transport.relay.Deliver(envelope, raw)
		return OutcomeUtils{}.FromSmtp(path, reply, err)
	}
}
