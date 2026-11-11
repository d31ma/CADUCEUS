import (
	"errors"
	"os"

	"mesail.dev/packages/events"
	"mesail.dev/packages/realtime"
)

// @RequestRoute("/api/v1/realtime/events")
// @Allow(Service)
type EventsController struct {
	// @Needs
	dataSource DataSource
}

// @PostRoute
func (controller *EventsController) PostEvent(
	// @FromHeader("authorization")
	authorization string,
	// @FromBody
	envelope EventEnvelope,
) (EventReceipt, error) {
	receipt, failed := controller.desk().Receive(authorization, controller.envelope(envelope))
	return EventReceipt{EventId: receipt.EventID, Status: receipt.Status}, controller.failure(failed)
}

func (controller *EventsController) desk() realtime.EventDesk {
	return realtime.DeskFactory{Environment: os.Getenv}.Events(StorageUtils{}.From(controller.dataSource))
}

func (controller *EventsController) envelope(envelope EventEnvelope) events.Envelope {
	return events.Envelope{EventID: envelope.EventId, Type: envelope.Type, DomainId: envelope.DomainId, Producer: envelope.Producer, OccurredAt: envelope.OccurredAt, Payload: envelope.Payload}
}

func (controller *EventsController) failure(failed error) error {
	invalid := realtime.Invalid{}
	envelope := events.InvalidEnvelope{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &envelope):
		return &InvalidRequest{Field: "envelope", Reason: envelope.Reason}
	case errors.As(failed, &events.NotSubscribed{}), errors.As(failed, &events.UnknownEventType{}):
		return &InvalidRequest{Field: "type", Reason: failed.Error()}
	case errors.As(failed, &invalid):
		return &InvalidRequest{Field: invalid.Field, Reason: invalid.Reason}
	case errors.Is(failed, realtime.Unauthenticated{}):
		return &Unauthenticated{}
	case errors.Is(failed, events.NotService{}), errors.Is(failed, realtime.Forbidden{}):
		return &Forbidden{}
	case errors.As(failed, &realtime.Missing{}):
		return &NotFound{}
	}
	return &Unavailable{}
}
