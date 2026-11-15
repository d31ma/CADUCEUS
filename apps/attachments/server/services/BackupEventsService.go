import "mesail.dev/packages/events"

// @Service
type BackupEventsService struct {
	// @Needs
	desks *BackupDeskService
	// @Needs
	failures *BackupFailureService
}

func (service *BackupEventsService) Receive(authorization string, request EventEnvelope) (EventReceipt, error) {
	envelope := events.Envelope{
		EventID: request.EventId, Type: request.Type, DomainId: request.DomainId,
		Producer: request.Producer, OccurredAt: request.OccurredAt, Payload: request.Payload,
	}
	receipt, failed := service.desks.EventDesk().Receive(authorization, envelope)
	return EventReceipt{EventId: receipt.EventID, Status: receipt.Status}, service.failures.TranslatedEvent(failed)
}
