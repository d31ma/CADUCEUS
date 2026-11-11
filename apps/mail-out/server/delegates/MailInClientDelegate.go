import "strings"

// @Delegate
// @NeedsAll()
type MailInClientDelegate struct {
	settings *SettingsDelegate
	client   *ServiceClientDelegate
}

func (MailInClientDelegate) route() string { return "/api/v1/mail-in/messages" }

func (mailIn *MailInClientDelegate) HandOff(envelope InboundEnvelope, realm string, authorize func(string) (string, error)) (int, error) {
	address := strings.TrimRight(mailIn.settings.Settings().MailInUrl, "/") + mailIn.route()
	return mailIn.client.Post(address, realm, authorize, envelope)
}
