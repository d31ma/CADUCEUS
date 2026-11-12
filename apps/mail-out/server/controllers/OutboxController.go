// @RequestRoute("/api/v1/mail-out/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	relay *OutboxService
}

// @PostRoute("/relay")
func (controller *OutboxController) PostRelay() (RelayReport, error) {
	return controller.relay.Relay()
}

// @GetRoute("/pending")
func (controller *OutboxController) GetPending() ([]PendingDocument, error) {
	return controller.relay.Pending()
}

// @GetRoute("/dead")
func (controller *OutboxController) GetDead() ([]DeadLetterView, error) {
	return controller.relay.Dead()
}

// @PostRoute("/dead/requeue")
func (controller *OutboxController) PostRequeue(
	// @FromBody
	request RequeueRequest,
) (RelayReport, error) {
	return controller.relay.Requeue(request)
}
