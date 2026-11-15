// @RequestRoute("/api/v1/attachments/events")
// @Allow(Service)
type EventsController struct {
	// @Needs
	events *BackupEventsService
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
