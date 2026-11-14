import (
	"mesail.dev/packages/devicebox"
	"mesail.dev/packages/outbox"
)

// @RequestRoute("/api/v1/devicebox/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	outbox *OutboxService
}

// @PostRoute("/relay")
func (controller *OutboxController) PostRelay() (outbox.Report, error) {
	return controller.outbox.Relay()
}

// @GetRoute("/dead")
func (controller *OutboxController) GetDead() ([]devicebox.DeadEvent, error) {
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
