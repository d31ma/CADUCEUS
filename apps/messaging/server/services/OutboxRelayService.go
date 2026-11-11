import (
	"cmp"
	"errors"
	"time"

	"mesail.dev/packages/adapters"
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

// @Service
type OutboxRelayService struct {
	// @Needs
	store *OutboxStoreService
	// @Needs
	settings *SettingsService
	pusher   outbox.Pusher
}

func (service *OutboxRelayService) RunOnce() (outbox.Report, error) {
	settings, failure := service.settings.Outbox()
	report, runFailure := service.runUnless(failure, settings)
	return report, cmp.Or(failure, runFailure)
}

func (service *OutboxRelayService) Drain(collection string, id string) {
	settings, failure := service.settings.Outbox()
	service.drainUnless(failure, settings, outbox.Ref{Collection: collection, Id: id})
}

func (service *OutboxRelayService) DeadEvents() ([]DeadEvent, error) {
	documents, failure := service.store.Dead()
	found, listFailure := service.deadEventsOf(documents)
	return found, cmp.Or(failure, listFailure)
}

func (service *OutboxRelayService) RetryDead(request RetryRequest) error {
	ref := outbox.Ref{Collection: request.Collection, Id: request.DocumentId}
	fields, failure := service.store.Load(ref)
	requeued, requeueFailure := outbox.DeadLetters{}.Requeue(fields, request.EventId, request.Subscriber)
	return service.saveUnless(cmp.Or(failure, service.missingLetter(requeueFailure)), ref, requeued)
}

func (service *OutboxRelayService) relay(settings outbox.Settings) outbox.Relay {
	return outbox.NewRelay(service.store, service.pusherOrDefault(), settings, time.Now)
}

func (service *OutboxRelayService) runUnless(failure error, settings outbox.Settings) (outbox.Report, error) {
	if failure != nil {
		return outbox.Report{}, nil
	}
	return service.relay(settings).RunOnce(service.sweepSize())
}

func (service *OutboxRelayService) drainUnless(failure error, settings outbox.Settings, ref outbox.Ref) {
	if failure == nil {
		service.relay(settings).RelayDocument(ref)
	}
}

func (service *OutboxRelayService) saveUnless(failure error, ref outbox.Ref, fields outbox.Fields) error {
	if failure != nil {
		return failure
	}
	return service.store.Save(ref, fields)
}

func (service *OutboxRelayService) pusherOrDefault() outbox.Pusher {
	if service.pusher != nil {
		return service.pusher
	}
	return adapters.NewDomainPusher(oidc.ServiceTokensFactory{}.Configured(), outbox.HTTPPusher{}.DefaultTimeout())
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

func (service *OutboxRelayService) missingLetter(failure error) error {
	if failure != nil {
		return &core.NotFound{Message: "no such dead event"}
	}
	return nil
}

func (service *OutboxRelayService) sweepSize() int {
	return 20
}
