import (
	"mesail.dev/packages/outbox"
)

// @RequestRoute("/api/v1/mail-in/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	relay *OutboxService
}

// @PostRoute("/relay")
func (controller *OutboxController) PostRelay() (outbox.Report, error) {
	return controller.relay.Relay()
}

// @GetRoute("/dead")
func (controller *OutboxController) GetDead() ([]DeadEvent, error) {
	return controller.relay.Dead()
}

// @PostRoute("/dead/requeue")
// @ResponseStatus(204)
func (controller *OutboxController) PostRetry(
	// @FromBody
	request RetryRequest,
) error {
	return controller.relay.Retry(request)
}
