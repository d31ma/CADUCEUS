package inbound

import "mesail.dev/packages/core"

type EmailDelivery struct {
	stores core.Stores
	ports  core.Ports
}

func NewEmailDelivery(stores core.Stores, ports core.Ports) EmailDelivery {
	return EmailDelivery{stores: stores, ports: ports}
}

func (delivery EmailDelivery) Deliver(email ReceivedEmail) error {
	key, matched, err := NewThreadMatcher(delivery.stores.ThreadKeys).Find(email.DomainId, email.Mail.Headers)
	if err != nil {
		return err
	}
	content, err := delivery.ports.Inbound.Content(email.DomainId, email.Mail.InboundId)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	placement, err := delivery.place(email, key, matched)
	if err != nil {
		return err
	}
	message, err := EmailMessages{}.New(email, placement, content, delivery.ports.Now())
	if err != nil {
		return err
	}
	message.Id, err = delivery.stores.Messages.Insert(message)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	return NewMessageFinisher(delivery.stores, delivery.ports).Finish(placement, message)
}

func (delivery EmailDelivery) place(email ReceivedEmail, key core.ThreadKey, matched bool) (Placement, error) {
	if matched {
		return NewThreadJoin(delivery.stores, delivery.ports).Join(email, key)
	}
	return NewConversationStarter(delivery.stores, delivery.ports).Start(email)
}
