package events

import (
	"slices"

	"mesail.dev/packages/oidc"
)

type Receiver struct {
	service string
	seen    SeenStore
	handler Handler
}

func NewReceiver(service string, seen SeenStore, handler Handler) Receiver {
	return Receiver{service: service, seen: seen, handler: handler}
}

func (receiver Receiver) Receive(caller oidc.Caller, envelope Envelope) (Receipt, error) {
	err := receiver.requireService(caller)
	if err != nil {
		return Receipt{}, err
	}
	err = EnvelopeCheck{}.Check(envelope)
	if err != nil {
		return Receipt{}, err
	}
	err = receiver.requireSubscribed(envelope.Type)
	if err != nil {
		return Receipt{}, err
	}
	seen, err := receiver.seen.Seen(envelope.DomainId, envelope.EventID)
	if err != nil {
		return Receipt{}, err
	}
	return receiver.settle(envelope, seen)
}

func (Receiver) serviceRole() string {
	return "Service"
}

func (receiver Receiver) requireService(caller oidc.Caller) error {
	if !slices.Contains(caller.Roles, receiver.serviceRole()) {
		return NotService{}
	}
	return nil
}

func (receiver Receiver) requireSubscribed(eventType string) error {
	if !slices.Contains(Catalog{}.ConsumersOf(eventType), receiver.service) {
		return NotSubscribed{Service: receiver.service, Type: eventType}
	}
	return nil
}

func (receiver Receiver) settle(envelope Envelope, seen bool) (Receipt, error) {
	if seen {
		return Receipt{EventID: envelope.EventID, Status: ReceiptStatus{}.Duplicate()}, nil
	}
	return receiver.apply(envelope)
}

func (receiver Receiver) apply(envelope Envelope) (Receipt, error) {
	err := receiver.handler.Handle(envelope)
	if err != nil {
		return Receipt{}, err
	}
	err = receiver.seen.Remember(envelope.DomainId, envelope.EventID)
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{EventID: envelope.EventID, Status: ReceiptStatus{}.Applied()}, nil
}
