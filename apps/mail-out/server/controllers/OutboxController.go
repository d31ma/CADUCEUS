import "mesail.dev/packages/oidc"

// @RequestRoute("/api/v1/mail-out/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	relay *OutboxRelayService
}

func (OutboxController) limit() int { return 100 }

// @PostRoute("/relay")
func (controller *OutboxController) PostRelay() (RelayReport, error) {
	return controller.relay.RunOnce(controller.limit(), oidc.ServiceTokensFactory{}.Configured().Authorization)
}

// @GetRoute("/pending")
func (controller *OutboxController) GetPending() ([]PendingDocument, error) {
	return controller.relay.Pending(controller.limit())
}

// @GetRoute("/dead")
func (controller *OutboxController) GetDead() ([]DeadLetterView, error) {
	return controller.relay.DeadLetters(controller.limit())
}

// @PostRoute("/dead/requeue")
func (controller *OutboxController) PostRequeue(
	// @FromBody
	request RequeueRequest,
) (RelayReport, error) {
	return controller.relay.Requeue(request, oidc.ServiceTokensFactory{}.Configured().Authorization)
}
