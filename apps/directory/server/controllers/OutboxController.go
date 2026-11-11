import (
	"cmp"

	"mesail.dev/packages/outbox"
)

// @RequestRoute("/api/v1/directory/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	callers *CallerService
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
	caller, refused := controller.callers.Caller(authorization)
	return cmp.Or(refused, controller.callers.Operator(caller))
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
	return controller.relay.Retry(request)
}
