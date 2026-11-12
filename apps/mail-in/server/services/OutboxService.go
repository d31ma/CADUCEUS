import (
	"mesail.dev/packages/mailin"
	"mesail.dev/packages/outbox"
)

// @Service
type OutboxService struct {
	// @Needs
	workflows *WorkflowService
	// @Needs
	failures *FailureService
}

func (service *OutboxService) Relay() (outbox.Report, error) {
	report, failed := service.workflows.Outbox().RunOnce()
	return report, service.failures.Translate(failed)
}

func (service *OutboxService) Dead() ([]DeadEvent, error) {
	dead, failed := service.workflows.Outbox().DeadEvents()
	return service.deadEvents(dead), service.failures.Translate(failed)
}

func (service *OutboxService) Retry(request RetryRequest) error {
	retry := mailin.RetryRequest{Collection: request.Collection, DocumentId: request.DocumentId, EventId: request.EventId, Subscriber: request.Subscriber}
	return service.failures.Translate(service.workflows.Outbox().Retry(retry))
}

func (service *OutboxService) deadEvents(dead []mailin.DeadEvent) []DeadEvent {
	views := []DeadEvent{}
	for _, event := range dead {
		views = append(views, DeadEvent{
			Collection: event.Collection,
			DocumentId: event.DocumentId,
			EventId:    event.EventId,
			Type:       event.Type,
			Subscriber: event.Subscriber,
			Attempts:   event.Attempts,
			LastError:  event.LastError,
			DeadAt:     event.DeadAt,
		})
	}
	return views
}
