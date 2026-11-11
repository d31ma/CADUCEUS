import (
	"cmp"
	"errors"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

// @Service
type EventsService struct {
	// @Needs
	callers *CallerService
	// @Needs
	seen *SeenEventsService
	// @Needs
	domains *DomainRegistryService
	// @Needs
	handlers *EventHandlersService
}

func (service *EventsService) Receive(authorization string, request EventEnvelope) (EventReceipt, error) {
	caller, failure := service.callers.Caller(authorization)
	envelope := service.envelope(request)
	failure = cmp.Or(failure, service.domainCheck(caller.DomainId, envelope.DomainId))
	receipt, failure := service.receiveUnless(failure, caller, envelope)
	return EventReceipt{EventId: receipt.EventID, Status: receipt.Status}, service.translated(failure)
}

func (service *EventsService) Handle(envelope events.Envelope) error {
	failure := service.domains.Register(envelope.DomainId)
	return service.dispatchUnless(failure, envelope)
}

func (service *EventsService) envelope(request EventEnvelope) events.Envelope {
	return events.Envelope{
		EventID:    request.EventId,
		Type:       request.Type,
		DomainId:   request.DomainId,
		Producer:   request.Producer,
		OccurredAt: request.OccurredAt,
		Payload:    request.Payload,
	}
}

func (service *EventsService) domainCheck(callerDomainId string, envelopeDomainId string) error {
	if callerDomainId != envelopeDomainId && callerDomainId != service.callers.OperatorRealm() {
		return &NotFound{What: "domain"}
	}
	return nil
}

func (service *EventsService) receiveUnless(failure error, caller oidc.Caller, envelope events.Envelope) (events.Receipt, error) {
	if failure != nil {
		return events.Receipt{}, failure
	}
	return events.NewReceiver(events.Services{}.Notify(), service.seen, service).Receive(caller, envelope)
}

func (service *EventsService) dispatchUnless(failure error, envelope events.Envelope) error {
	if failure != nil {
		return failure
	}
	return service.handlers.Dispatch(envelope)
}

func (service *EventsService) translated(failure error) error {
	switch {
	case failure == nil:
		return nil
	case errors.As(failure, &events.NotService{}):
		return &NotService{}
	case errors.As(failure, new(*Unauthenticated)), errors.As(failure, new(*NotFound)), errors.As(failure, new(*StorageUnavailable)):
		return failure
	}
	return &InvalidRequest{Field: "envelope", Reason: failure.Error()}
}
