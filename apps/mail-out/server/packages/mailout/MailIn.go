package mailout

import "strings"

type MailIn struct {
	Client  ServiceClient
	BaseUrl string
}

func (mailIn MailIn) HandOff(envelope InboundEnvelope, realm string) (int, error) {
	return mailIn.Client.Post(strings.TrimRight(mailIn.BaseUrl, "/")+"/api/v1/mail-in/messages", realm, envelope)
}
