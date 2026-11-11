import (
	"errors"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @RequestRoute("/api/v1/mail-out/events")
// @Allow(Service)
type EventsController struct {
	// @Needs
	events *EventsService
}

// @PostRoute
func (controller *EventsController) PostEvent(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	envelope SendEnvelope,
) (EventReceipt, error) {
	caller, _ := oidc.Configured().Identify(authorization)
	receipt, err := controller.events.ReceiveSend(caller, envelope, oidc.ServiceTokensFactory{}.Configured().Authorization)
	return receipt, controller.answer(err)
}

func (controller *EventsController) answer(failure error) error {
	invalid, unsubscribed, notService, missing := events.InvalidEnvelope{}, events.NotSubscribed{}, events.NotService{}, outbox.NotFound{}
	switch {
	case errors.As(failure, &notService):
		return &ForbiddenCaller{}
	case errors.As(failure, &missing):
		return &UnknownDomain{}
	case errors.As(failure, &invalid), errors.As(failure, &unsubscribed):
		return &RejectedEvent{Reason: failure.Error()}
	default:
		return failure
	}
}
