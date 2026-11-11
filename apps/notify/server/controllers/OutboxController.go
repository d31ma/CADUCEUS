// @RequestRoute("/api/v1/notify/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	relay *OutboxRelayService
}

// @PostRoute("/relay")
func (controller *OutboxController) PostRelay(
	// @FromHeader("authorization")
	authorization string,
) (OutboxReport, error) {
	return controller.relay.RunOnce(authorization)
}

// @GetRoute("/dead")
func (controller *OutboxController) GetDead(
	// @FromHeader("authorization")
	authorization string,
) ([]DeadEvent, error) {
	return controller.relay.DeadEvents(authorization)
}

// @PostRoute("/dead/requeue")
// @ResponseStatus(204)
func (controller *OutboxController) PostRetry(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request RetryRequest,
) error {
	return controller.relay.RetryDead(authorization, request)
}
