import (
	"cmp"
	"errors"
	"time"

	"mesail.dev/packages/outbox"
)

// @Service
type OutboxRelayService struct {
	// @Needs
	store *OutboxStoreService
	// @Needs
	settings *SettingsService
	// @Needs
	domains *DomainStorageService
}

func (service *OutboxRelayService) RunOnce() (outbox.Report, error) {
	settings, failure := service.settings.Outbox()
	domains, domainsFailure := service.domains.All()
	report, runFailure := service.sweep(settings, domains)
	return report, cmp.Or(service.missingSettings(failure), domainsFailure, runFailure)
}

func (service *OutboxRelayService) Drain(domainId string, ref outbox.Ref) {
	settings, failure := service.settings.Outbox()
	service.drainUnless(cmp.Or(failure, service.unsaved(ref)), service.relay(settings, domainId), ref)
}

func (service *OutboxRelayService) DeadEvents() ([]DeadEvent, error) {
	documents, failure := service.store.Dead()
	found, listFailure := service.deadEventsOf(documents)
	return found, cmp.Or(failure, listFailure)
}

func (service *OutboxRelayService) Retry(request RetryRequest) error {
	ref := outbox.Ref{Collection: request.Collection, Id: request.DocumentId}
	fields, failure := service.store.Load(ref)
	retried, retryFailure := outbox.DeadLetters{}.Requeue(fields, request.EventId, request.Subscriber)
	return service.saveUnless(cmp.Or(failure, service.missingLetter(retryFailure)), ref, retried)
}

func (service *OutboxRelayService) relay(settings outbox.Settings, domainId string) outbox.Relay {
	pusher := outbox.NewHTTPPusher(DomainServiceToken{DomainId: domainId}, outbox.HTTPPusher{}.DefaultTimeout())
	pending := func(limit int) ([]outbox.Document, error) { return service.store.PendingIn(domainId, limit) }
	store := OutboxStore{PendingOf: pending, LoadOf: service.store.Load, SaveOf: service.store.Save}
	return outbox.NewRelay(store, pusher, settings, time.Now)
}

func (service *OutboxRelayService) sweep(settings outbox.Settings, domains []DomainDocument) (outbox.Report, error) {
	total := outbox.Report{}
	failures := []error{}
	deadline := time.Now().Add(service.timeBudget())
	for _, domain := range domains {
		report, failure := service.relay(settings, domain.DomainId).RunWithin(max(service.sweepSize()-total.Documents, 0), time.Until(deadline))
		total = outbox.Report{Documents: total.Documents + report.Documents, Delivered: total.Delivered + report.Delivered, Retried: total.Retried + report.Retried, DeadLettered: total.DeadLettered + report.DeadLettered, More: cmp.Or(total.More, report.More)}
		failures = append(failures, failure)
	}
	return total, errors.Join(failures...)
}

func (service *OutboxRelayService) drainUnless(failure error, relay outbox.Relay, ref outbox.Ref) {
	if failure == nil {
		relay.RelayDocument(ref)
	}
}

func (service *OutboxRelayService) saveUnless(failure error, ref outbox.Ref, fields outbox.Fields) error {
	if failure != nil {
		return failure
	}
	return service.store.Save(ref, fields)
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

func (service *OutboxRelayService) unsaved(ref outbox.Ref) error {
	if ref.Id == "" {
		return errors.New("nothing was written")
	}
	return nil
}

func (service *OutboxRelayService) missingSettings(failure error) error {
	if failure != nil {
		return &Unavailable{Message: "the outbox is not configured", Cause: failure}
	}
	return nil
}

func (service *OutboxRelayService) missingLetter(failure error) error {
	if failure != nil {
		return &NotFound{Message: "no such dead event"}
	}
	return nil
}

func (service *OutboxRelayService) timeBudget() time.Duration {
	return 3 * time.Second
}

func (service *OutboxRelayService) sweepSize() int {
	return 10
}
