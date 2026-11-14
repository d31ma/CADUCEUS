import "mesail.dev/packages/events"

// @RequestRoute("/api/v1/messaging/events")
// @Allow(Service)
type EventsController struct {
	// @Needs
	events *EventRouteService
}

// @PostRoute
func (controller *EventsController) PostEvent(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	envelope EventEnvelope,
) (events.Receipt, error) {
	return controller.events.Receive(authorization, envelope)
}
