import (
	"context"
	"net"
	"strconv"

	"mesail.dev/packages/smtpclient"
)

// @Delegate
// @NeedsAll()
type SmtpDirectDelegate struct {
	settings *SettingsDelegate
}

func (direct *SmtpDirectDelegate) Deliver(envelope smtpclient.Envelope, message []byte) (smtpclient.Reply, error) {
	delivery := direct.delivery(direct.settings.Settings())
	deadline, cancel := context.WithTimeout(context.Background(), direct.settings.Timeout())
	defer cancel()
	return delivery.Deliver(deadline, envelope, message)
}

func (direct *SmtpDirectDelegate) delivery(settings MailOutSettings) smtpclient.Delivery {
	dialer := smtpclient.DirectDialer{Timeout: direct.settings.Timeout()}
	if settings.DirectMxHost != "" {
		return smtpclient.RelayDelivery{Address: net.JoinHostPort(settings.DirectMxHost, strconv.Itoa(settings.DirectPort)), Dialer: dialer, HeloName: settings.HeloName}
	}
	return smtpclient.DirectDelivery{Hosts: smtpclient.MxHosts{Resolver: net.DefaultResolver}, Dialer: dialer, HeloName: settings.HeloName, Port: settings.DirectPort}
}
