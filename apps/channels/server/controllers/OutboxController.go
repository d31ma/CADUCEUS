import "mesail.dev/packages/channels"

// @RequestRoute("/api/v1/channels/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	outbox *OutboxService
}

// @PostRoute("/relay")
func (controller *OutboxController) PostRelay() (channels.SweepReport, error) {
	return controller.outbox.Relay()
}

// @GetRoute("/pending")
func (controller *OutboxController) GetPending() (OutboxCount, error) {
	return controller.outbox.Pending()
}

// @GetRoute("/dead")
func (controller *OutboxController) GetDead() ([]channels.DeadEvent, error) {
	return controller.outbox.Dead()
}

// @PostRoute("/dead/requeue")
// @ResponseStatus(204)
func (controller *OutboxController) PostRetry(
	// @FromBody
	request RetryRequest,
) error {
	return controller.outbox.Retry(request)
}
