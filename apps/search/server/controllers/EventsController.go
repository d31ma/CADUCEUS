// @RequestRoute("/api/v1/search/events")
// @Allow(Service)
type EventsController struct {
	// @Needs
	events *EventService
}

// @PostRoute
func (controller *EventsController) PostEvent(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	envelope EventEnvelope,
) (EventReceipt, error) {
	return controller.events.Receive(authorization, envelope)
}
