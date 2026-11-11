package mailout

import (
	"context"
	"time"

	"mesail.dev/packages/smtpclient"
)

type Transport struct {
	Directory     Directory
	MailIn        MailIn
	Relay         smtpclient.Delivery
	Direct        smtpclient.Delivery
	DirectEnabled bool
	Timeout       time.Duration
}

func (Transport) installWideRealm() string { return "mesail-operators" }

func (transport Transport) Deliver(item Item, raw []byte) Outcome {
	recipientDomain := Addresses{}.Domain(item.Rcpt)
	domain, found, err := transport.Directory.Domain(recipientDomain, transport.installWideRealm())
	if err != nil {
		return Outcome{Status: States{}.Deferred(), Transport: "directory", Detail: "the directory did not answer"}
	}
	if found && domain.State == "active" {
		return OutcomeRules{}.FromHandOff(transport.MailIn.HandOff(HandOff{}.Envelope(item), recipientDomain))
	}
	return transport.smtp(item, raw)
}

func (transport Transport) smtp(item Item, raw []byte) Outcome {
	name, delivery := "relay", transport.Relay
	if transport.DirectEnabled {
		name, delivery = "direct", transport.Direct
	}
	deadline, cancel := context.WithTimeout(context.Background(), transport.Timeout)
	defer cancel()
	reply, err := delivery.Deliver(deadline, smtpclient.Envelope{From: item.From, Recipients: []string{item.Rcpt}}, raw)
	return OutcomeRules{}.FromSmtp(name, reply, err)
}
