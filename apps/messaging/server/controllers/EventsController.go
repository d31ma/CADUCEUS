import (
	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

// @RequestRoute("/api/v1/messaging/events")
// @Allow(Service)
type EventsController struct {
	// @Needs
	callers *CallerService
	// @Needs
	events *EventsService
	// @Needs
	wire *WireService
}

// @PostRoute
func (controller *EventsController) PostEvent(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	envelope EventEnvelope,
) (events.Receipt, error) {
	caller, refused := controller.callers.ServiceCaller(authorization)
	return controller.received(caller, controller.wire.Envelope(envelope), refused)
}

func (controller *EventsController) received(caller oidc.Caller, envelope events.Envelope, failure error) (events.Receipt, error) {
	if failure != nil {
		return events.Receipt{}, failure
	}
	return controller.events.Receive(caller, envelope)
}
