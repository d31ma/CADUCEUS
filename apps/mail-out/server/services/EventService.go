import (
	"errors"

	"mesail.dev/packages/events"
	"mesail.dev/packages/mailout"
)

// @Service
type EventService struct {
	// @Needs
	desks *DeskService
}

func (service *EventService) Receive(authorization string, envelope SendEnvelope) (EventReceipt, error) {
	receipt, failed := service.desks.Desk().Receive(authorization, envelope)
	return EventReceipt{EventId: receipt.EventID, Status: receipt.Status}, service.failure(failed)
}

func (service *EventService) failure(failed error) error {
	invalid, unsubscribed, notService, mismatch, refused := events.InvalidEnvelope{}, events.NotSubscribed{}, events.NotService{}, mailout.DomainMismatch{}, mailout.InvalidSendRequest{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &notService):
		return &ForbiddenCaller{ErrorNumber: notService.Number()}
	case errors.As(failed, &mismatch):
		return &UnknownDomain{ErrorNumber: mismatch.Number()}
	case errors.As(failed, &refused):
		return RejectedEvent{}.SendRequest(refused.Error(), refused.Number())
	case errors.As(failed, &invalid):
		return RejectedEvent{}.Kit(invalid.Error(), invalid.Number())
	case errors.As(failed, &unsubscribed):
		return RejectedEvent{}.Kit(unsubscribed.Error(), unsubscribed.Number())
	default:
		return failed
	}
}
