import "mesail.dev/packages/outbox"

// @RequestRoute("/api/v1/messaging/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	outbox *OutboxRouteService
}

// @PostRoute("/relay")
func (controller *OutboxController) PostRelay(
	// @FromHeader("authorization")
	authorization string,
) (outbox.Report, error) {
	return controller.outbox.Relay(authorization)
}

// @GetRoute("/dead")
func (controller *OutboxController) GetDead(
	// @FromHeader("authorization")
	authorization string,
) ([]DeadEvent, error) {
	return controller.outbox.Dead(authorization)
}

// @PostRoute("/dead/requeue")
// @ResponseStatus(204)
func (controller *OutboxController) PostRetry(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request RetryRequest,
) error {
	return controller.outbox.Retry(authorization, request)
}
