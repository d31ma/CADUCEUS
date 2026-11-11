import "mesail.dev/packages/events"

// @Service
type EventHandlersService struct {
	// @Needs
	handlers *NotificationEventsService
}

func (service *EventHandlersService) Dispatch(envelope events.Envelope) error {
	return service.route(envelope)
}

func (service *EventHandlersService) route(envelope events.Envelope) error {
	types := events.Types{}
	switch envelope.Type {
	case types.MemberUpserted():
		return service.handlers.MemberUpserted(envelope)
	case types.ReadPositionMoved():
		return service.handlers.ReadPositionMoved(envelope)
	case types.MessageCreated():
		return service.handlers.MessageCreated(envelope)
	case types.MessageDeleted():
		return service.handlers.MessageDeleted(envelope)
	case types.MessageEdited(), types.ReactionChanged():
		return nil
	}
	return &InvalidRequest{Field: "type", Reason: "notify does not handle events of type " + envelope.Type}
}
