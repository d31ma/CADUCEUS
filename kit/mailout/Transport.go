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
	return transport.send(item, raw, transport.hosted(domain, found), err)
}

func (Transport) hosted(domain Domain, found bool) bool {
	return found && domain.IsActive()
}

func (transport Transport) send(item Item, raw []byte, hosted bool, lookupErr error) Outcome {
	switch {
	case lookupErr != nil:
		return Outcome{Status: States{}.Deferred(), Transport: "directory", Detail: "the directory did not answer"}
	case hosted:
		return transport.handOff(item)
	default:
		return transport.smtp(item, raw)
	}
}

func (transport Transport) handOff(item Item) Outcome {
	return OutcomeRules{}.FromHandOff(transport.MailIn.HandOff(HandOff{}.Envelope(item), Addresses{}.Domain(item.Rcpt)))
}

func (transport Transport) smtp(item Item, raw []byte) Outcome {
	name, delivery := transport.route()
	deadline, cancel := context.WithTimeout(context.Background(), transport.Timeout)
	defer cancel()
	reply, err := delivery.Deliver(deadline, smtpclient.Envelope{From: item.From, Recipients: []string{item.Rcpt}}, raw)
	return OutcomeRules{}.FromSmtp(name, reply, err)
}

func (transport Transport) route() (string, smtpclient.Delivery) {
	if transport.DirectEnabled {
		return "direct", transport.Direct
	}
	return "relay", transport.Relay
}
