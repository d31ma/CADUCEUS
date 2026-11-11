import "mesail.dev/packages/outbox"

// @RequestRoute("/api/v1/mail-in/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	relay *OutboxRelayService
}

// @PostRoute("/relay")
func (controller *OutboxController) PostRelay() (outbox.Report, error) {
	return controller.relay.RunOnce()
}

// @GetRoute("/dead")
func (controller *OutboxController) GetDead() ([]DeadEvent, error) {
	return controller.relay.DeadEvents()
}

// @PostRoute("/dead/requeue")
// @ResponseStatus(204)
func (controller *OutboxController) PostRequeue(
	// @FromBody
	request RequeueRequest,
) error {
	return controller.relay.Requeue(request)
}
