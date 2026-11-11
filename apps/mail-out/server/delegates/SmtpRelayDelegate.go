import (
	"context"

	"mesail.dev/packages/smtpclient"
)

// @Delegate
// @NeedsAll()
type SmtpRelayDelegate struct {
	settings *SettingsDelegate
}

func (relay *SmtpRelayDelegate) Deliver(envelope smtpclient.Envelope, message []byte) (smtpclient.Reply, error) {
	settings := relay.settings.Settings()
	dialer, err := smtpclient.ProxyDialers{}.FromEnvironment()
	delivery := smtpclient.RelayDelivery{Address: settings.RelayAddress, Dialer: dialer, HeloName: settings.HeloName, Username: settings.RelayUsername, Password: settings.RelayPassword, RequireTls: settings.RelayRequireTls, ImplicitTls: settings.RelayImplicitTls}
	return relay.deliver(delivery, envelope, message, err)
}

func (relay *SmtpRelayDelegate) deliver(delivery smtpclient.RelayDelivery, envelope smtpclient.Envelope, message []byte, failure error) (smtpclient.Reply, error) {
	if failure != nil {
		return smtpclient.Reply{}, failure
	}
	deadline, cancel := context.WithTimeout(context.Background(), relay.settings.Timeout())
	defer cancel()
	return delivery.Deliver(deadline, envelope, message)
}
