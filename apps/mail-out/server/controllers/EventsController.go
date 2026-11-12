// @RequestRoute("/api/v1/mail-out/events")
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
	envelope SendEnvelope,
) (EventReceipt, error) {
	return controller.events.Receive(authorization, envelope)
}
