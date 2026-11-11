import (
	"cmp"
	"errors"

	"mesail.dev/packages/outbox"
)

// @Service
type OutboxRelayService struct {
	// @Needs
	storage *StorageService
	// @Needs
	failures *FailureService
}

func (service *OutboxRelayService) RunOnce() (outbox.Report, error) {
	report, failure := service.storage.Storage().Outbox().RunOnce(service.sweepSize())
	return report, service.failures.Known(failure)
}

func (service *OutboxRelayService) DeadEvents() ([]DeadEvent, error) {
	documents, failure := service.storage.Storage().Outbox().Dead()
	found, listFailure := service.deadEventsOf(documents)
	return found, service.failures.Known(cmp.Or(failure, listFailure))
}

func (service *OutboxRelayService) RetryDead(request RetryRequest) error {
	ref := outbox.Ref{Collection: request.Collection, Id: request.DocumentId}
	return service.failures.Known(service.storage.Storage().Outbox().Retry(ref, request.EventId, request.Subscriber))
}

func (service *OutboxRelayService) deadEventsOf(documents []outbox.Document) ([]DeadEvent, error) {
	found := []DeadEvent{}
	failures := []error{}
	for _, document := range documents {
		letters, failure := outbox.DeadLetters{}.List(document.Fields)
		found, failures = append(found, service.views(document.Ref, letters)...), append(failures, failure)
	}
	return found, errors.Join(failures...)
}

func (service *OutboxRelayService) views(ref outbox.Ref, letters []outbox.DeadLetter) []DeadEvent {
	views := []DeadEvent{}
	for _, letter := range letters {
		views = append(views, DeadEvent{Collection: ref.Collection, DocumentId: ref.Id, EventId: letter.Envelope.EventID, Type: letter.Envelope.Type, Subscriber: letter.Subscriber, Attempts: letter.Attempts, LastError: letter.LastError, DeadAt: letter.DeadAt})
	}
	return views
}

func (service *OutboxRelayService) sweepSize() int {
	return 20
}
