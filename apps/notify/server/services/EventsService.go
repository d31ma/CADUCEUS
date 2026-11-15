import (
	"cmp"
	"errors"

	"mesail.dev/packages/events"
	"mesail.dev/packages/notify"
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

func (service *EventsService) handle(envelope events.Envelope) error {
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
	if callerDomainId != envelopeDomainId && !service.callers.IsInstallWide(callerDomainId) {
		return NotFound{}.Domain()
	}
	return nil
}

func (service *EventsService) receiveUnless(failure error, caller oidc.Caller, envelope events.Envelope) (events.Receipt, error) {
	if failure != nil {
		return events.Receipt{}, failure
	}
	seen := notify.SeenStoreFunctions{SeenFunction: service.seen.Seen, RememberFunction: service.seen.Remember}
	return events.NewReceiver(events.Services{}.Notify(), seen, notify.EventHandlerFunction(service.handle)).Receive(caller, envelope)
}

func (service *EventsService) dispatchUnless(failure error, envelope events.Envelope) error {
	if failure != nil {
		return failure
	}
	return service.handlers.Dispatch(envelope)
}

func (service *EventsService) translated(failure error) error {
	return cmp.Or(service.notService(failure), service.refusedEnvelope(failure), service.unsubscribedType(failure), service.ownFailure(failure))
}

func (service *EventsService) envelopeField() string {
	return "envelope"
}

func (service *EventsService) notService(failure error) error {
	refused := events.NotService{}
	if !errors.As(failure, &refused) {
		return nil
	}
	return NotService{}.Kit(refused.Error(), refused.Number())
}

func (service *EventsService) refusedEnvelope(failure error) error {
	invalid := events.InvalidEnvelope{}
	if !errors.As(failure, &invalid) {
		return nil
	}
	return InvalidRequest{}.Kit(service.envelopeField(), invalid.Error(), invalid.Number())
}

func (service *EventsService) unsubscribedType(failure error) error {
	unsubscribed := events.NotSubscribed{}
	if !errors.As(failure, &unsubscribed) {
		return nil
	}
	return InvalidRequest{}.Kit(service.envelopeField(), unsubscribed.Error(), unsubscribed.Number())
}

func (service *EventsService) ownFailure(failure error) error {
	switch {
	case failure == nil:
		return nil
	case errors.As(failure, new(*Unauthenticated)), errors.As(failure, new(*NotFound)), errors.As(failure, new(*StorageUnavailable)):
		return failure
	}
	return InvalidRequest{}.BadEnvelope(failure.Error())
}
