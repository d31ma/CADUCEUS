import (
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @RequestRoute("/api/v1/messaging/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	relay *OutboxRelayService
}

// @PostRoute("/relay")
func (controller *OutboxController) PostRelay(
	// @FromHeader("authorization")
	authorization string,
) (outbox.Report, error) {
	return controller.relayUnless(controller.installWide(authorization))
}

// @GetRoute("/dead")
func (controller *OutboxController) GetDead(
	// @FromHeader("authorization")
	authorization string,
) ([]DeadEvent, error) {
	return controller.deadUnless(controller.installWide(authorization))
}

// @PostRoute("/dead/requeue")
// @ResponseStatus(204)
func (controller *OutboxController) PostRetry(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	request RetryRequest,
) error {
	return controller.retryUnless(controller.installWide(authorization), request)
}

func (controller *OutboxController) installWide(authorization string) error {
	return controller.operatorOnly(controller.fromOperatorRealm(authorization))
}

func (controller *OutboxController) fromOperatorRealm(authorization string) bool {
	caller, failure := oidc.Configured().Identify(authorization)
	return failure == nil && caller.DomainId == controller.operatorRealm()
}

func (controller *OutboxController) operatorOnly(fromOperatorRealm bool) error {
	if fromOperatorRealm {
		return nil
	}
	return &Forbidden{Message: "the outbox is managed from the operator realm only"}
}

func (controller *OutboxController) relayUnless(refused error) (outbox.Report, error) {
	if refused != nil {
		return outbox.Report{}, refused
	}
	return controller.relay.RunOnce()
}

func (controller *OutboxController) deadUnless(refused error) ([]DeadEvent, error) {
	if refused != nil {
		return nil, refused
	}
	return controller.relay.DeadEvents()
}

func (controller *OutboxController) retryUnless(refused error, request RetryRequest) error {
	if refused != nil {
		return refused
	}
	return controller.relay.RetryDead(request)
}

func (controller *OutboxController) operatorRealm() string {
	return "mesail-operators"
}
