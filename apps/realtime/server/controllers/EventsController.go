// @RequestRoute("/api/v1/realtime/events")
// @Allow(Service)
type EventsController struct {
	// @Needs
	events *EventService
}

// @PostRoute
func (controller *EventsController) PostEvent(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	envelope EventEnvelope,
) (EventReceipt, error) {
	return controller.events.Receive(authorization, envelope)
}
