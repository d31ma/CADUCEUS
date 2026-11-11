import (
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @Service
type EventsService struct {
	// @Needs
	seenEvents *SeenEventsDelegate
	// @Needs
	intake *EventIntakeService
}

func (EventsService) serviceRole() string { return "Service" }

func (service *EventsService) ReceiveSend(caller oidc.Caller, envelope SendEnvelope, authorize func(string) (string, error)) (EventReceipt, error) {
	payload, encodeErr := json.Marshal(envelope.Payload)
	receipt, err := service.receive(caller, events.Envelope{EventID: envelope.EventId, Type: envelope.Type, DomainId: envelope.DomainId, Producer: envelope.Producer, OccurredAt: envelope.OccurredAt, Payload: payload}, authorize)
	return EventReceipt{EventId: receipt.EventID, Status: receipt.Status}, errors.Join(encodeErr, err)
}

func (service *EventsService) receive(caller oidc.Caller, envelope events.Envelope, authorize func(string) (string, error)) (events.Receipt, error) {
	admitErr := errors.Join(service.requireService(caller), service.requireDomain(caller, envelope), events.EnvelopeCheck{}.Check(envelope), service.requireConsumer(envelope.Type))
	seen, seenErr := service.seen(envelope, admitErr)
	applyErr := service.apply(envelope, seen, authorize, errors.Join(admitErr, seenErr))
	rememberErr := service.remember(envelope, seen, applyErr)
	return service.receipt(envelope, seen), rememberErr
}

func (service *EventsService) requireService(caller oidc.Caller) error {
	if !slices.Contains(caller.Roles, service.serviceRole()) {
		return events.NotService{}
	}
	return nil
}

func (EventsService) requireDomain(caller oidc.Caller, envelope events.Envelope) error {
	if !strings.EqualFold(caller.DomainId, envelope.DomainId) {
		return outbox.NotFound{Kind: "domain", Key: envelope.DomainId}
	}
	return nil
}

func (EventsService) requireConsumer(eventType string) error {
	mailOut := events.Services{}.MailOut()
	if !slices.Contains(events.Catalog{}.ConsumersOf(eventType), mailOut) {
		return events.NotSubscribed{Service: mailOut, Type: eventType}
	}
	return nil
}

func (service *EventsService) seen(envelope events.Envelope, failure error) (bool, error) {
	if failure != nil {
		return false, failure
	}
	return service.seenEvents.Seen(envelope.DomainId, envelope.EventID)
}

func (service *EventsService) apply(envelope events.Envelope, seen bool, authorize func(string) (string, error), failure error) error {
	if seen || failure != nil {
		return failure
	}
	return service.intake.Accept(envelope, authorize)
}

func (service *EventsService) remember(envelope events.Envelope, seen bool, failure error) error {
	if seen || failure != nil {
		return failure
	}
	return service.seenEvents.Remember(envelope.DomainId, envelope.EventID)
}

func (EventsService) receipt(envelope events.Envelope, seen bool) events.Receipt {
	statuses := events.ReceiptStatus{}
	if seen {
		return events.Receipt{EventID: envelope.EventID, Status: statuses.Duplicate()}
	}
	return events.Receipt{EventID: envelope.EventID, Status: statuses.Applied()}
}
