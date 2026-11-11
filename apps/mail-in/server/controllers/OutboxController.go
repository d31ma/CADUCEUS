import (
	"errors"

	"mesail.dev/packages/mailin"
	"mesail.dev/packages/outbox"
)

// @RequestRoute("/api/v1/mail-in/outbox")
// @Allow(Service)
type OutboxController struct {
	// @Needs
	workflows *WorkflowService
}

// @PostRoute("/relay")
func (controller *OutboxController) PostRelay() (outbox.Report, error) {
	report, failed := controller.workflows.Outbox().RunOnce()
	return report, controller.failure(failed)
}

// @GetRoute("/dead")
func (controller *OutboxController) GetDead() ([]DeadEvent, error) {
	dead, failed := controller.workflows.Outbox().DeadEvents()
	return controller.deadEvents(dead), controller.failure(failed)
}

// @PostRoute("/dead/requeue")
// @ResponseStatus(204)
func (controller *OutboxController) PostRetry(
	// @FromBody
	request RetryRequest,
) error {
	retry := mailin.RetryRequest{Collection: request.Collection, DocumentId: request.DocumentId, EventId: request.EventId, Subscriber: request.Subscriber}
	return controller.failure(controller.workflows.Outbox().Retry(retry))
}

func (controller *OutboxController) deadEvents(dead []mailin.DeadEvent) []DeadEvent {
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

func (controller *OutboxController) failure(failed error) error {
	missing := mailin.Missing{}
	switch {
	case failed == nil:
		return nil
	case errors.As(failed, &missing):
		return &NotFound{Message: missing.Message}
	}
	return &StorageUnavailable{Operation: failed.Error()}
}
