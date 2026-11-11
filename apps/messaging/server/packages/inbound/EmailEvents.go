package inbound

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type EmailEvents struct {
	stores core.Stores
	ports  core.Ports
}

func NewEmailEvents(stores core.Stores, ports core.Ports) EmailEvents {
	return EmailEvents{stores: stores, ports: ports}
}

func (emailEvents EmailEvents) Received(envelope events.Envelope) error {
	mail := events.EmailReceived{}
	err := events.PayloadReader{}.Read(envelope, &mail)
	if err != nil {
		return err
	}
	email := ReceivedEmail{DomainId: envelope.DomainId, Mail: mail, SendKey: Digests{}.Hex(emailEvents.sendKeyPrefix() + mail.InboundId)}
	existing, found, err := emailEvents.stored(email, Months{}.CurrentAndPrevious(emailEvents.ports.Now()))
	if err != nil {
		return err
	}
	return emailEvents.deliverUnlessStored(email, existing, found)
}

func (EmailEvents) sendKeyPrefix() string {
	return "inbound\n"
}

func (emailEvents EmailEvents) stored(email ReceivedEmail, months []string) (core.Message, bool, error) {
	for _, month := range months {
		message, found, err := emailEvents.stores.Messages.BySendKey(email.DomainId, month, email.SendKey)
		if err != nil {
			return core.Message{}, false, Unavailability{}.Of(err)
		}
		if found {
			return message, true, nil
		}
	}
	return core.Message{}, false, nil
}

func (emailEvents EmailEvents) deliverUnlessStored(email ReceivedEmail, existing core.Message, found bool) error {
	if found {
		return NewEmailResume(emailEvents.stores, emailEvents.ports).Continue(existing)
	}
	return NewEmailDelivery(emailEvents.stores, emailEvents.ports).Deliver(email)
}
