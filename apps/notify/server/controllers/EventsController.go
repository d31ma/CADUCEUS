// @RequestRoute("/api/v1/notify/events")
// @Allow(Service)
type EventsController struct {
	// @Needs
	events *EventsService
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
