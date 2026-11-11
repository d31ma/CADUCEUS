import (
	"errors"
	"os"

	"mesail.dev/packages/events"
	"mesail.dev/packages/mailout"
)

// @RequestRoute("/api/v1/mail-out/events")
// @Allow(Service)
type EventsController struct {
	// @Needs
	dataSource DataSource
}

// @PostRoute
func (controller *EventsController) PostEvent(
	// @FromHeader("Authorization")
	authorization string,
	// @FromBody
	envelope SendEnvelope,
) (EventReceipt, error) {
	receipt, failed := mailout.DeskFactory{Environment: os.Getenv}.Desk(controller.dataSource).Receive(authorization, envelope)
	return EventReceipt{EventId: receipt.EventID, Status: receipt.Status}, controller.failure(failed)
}

func (controller *EventsController) failure(failed error) error {
	invalid, unsubscribed, notService, mismatch := events.InvalidEnvelope{}, events.NotSubscribed{}, events.NotService{}, mailout.DomainMismatch{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &notService):
		return &ForbiddenCaller{}
	case errors.As(failed, &mismatch):
		return &UnknownDomain{}
	case errors.As(failed, &invalid), errors.As(failed, &unsubscribed):
		return &RejectedEvent{Reason: failed.Error()}
	default:
		return failed
	}
}
