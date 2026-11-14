import (
	"os"

	"mesail.dev/packages/events"
	"mesail.dev/packages/realtime"
)

// @Service
type EventService struct {
	// @Needs
	dataSource DataSource
	// @Needs
	failures *ErrorMappingService
}

func (service *EventService) Receive(authorization string, envelope EventEnvelope) (EventReceipt, error) {
	receipt, failed := service.desk().Receive(authorization, service.envelope(envelope))
	return EventReceipt{EventId: receipt.EventID, Status: receipt.Status}, service.failures.EventError(failed)
}

func (service *EventService) desk() realtime.EventDesk {
	return realtime.DeskFactory{Environment: os.Getenv}.Events(StorageUtils{}.From(service.dataSource))
}

func (service *EventService) envelope(envelope EventEnvelope) events.Envelope {
	return events.Envelope{EventID: envelope.EventId, Type: envelope.Type, DomainId: envelope.DomainId, Producer: envelope.Producer, OccurredAt: envelope.OccurredAt, Payload: envelope.Payload}
}
