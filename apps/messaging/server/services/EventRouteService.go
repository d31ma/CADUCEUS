import (
	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

// @Service
type EventRouteService struct {
	// @Needs
	callers *CallerService
	// @Needs
	events *EventsService
	// @Needs
	wire *WireService
}

func (service *EventRouteService) Receive(authorization string, envelope EventEnvelope) (events.Receipt, error) {
	caller, refused := service.callers.ServiceCaller(authorization)
	return service.received(caller, service.wire.Envelope(envelope), refused)
}

func (service *EventRouteService) received(caller oidc.Caller, envelope events.Envelope, failure error) (events.Receipt, error) {
	if failure != nil {
		return events.Receipt{}, failure
	}
	return service.events.Receive(caller, envelope)
}
