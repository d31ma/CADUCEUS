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
	settings *NotifySettingsService
	// @Needs
	tokens *OperatorTokenService
	// @Needs
	callers *CallerService
	pusher  outbox.Pusher
}

func (service *OutboxRelayService) RunOnce(authorization string) (OutboxReport, error) {
	failure := service.callers.Operator(authorization)
	report, failure := service.runUnless(failure)
	return service.wire(report), service.unavailable(failure)
}

func (service *OutboxRelayService) RelayDocument(collection string, id string) (OutboxReport, error) {
	relay, failure := service.relay()
	report, problem := relay.RelayDocument(outbox.Ref{Collection: collection, Id: id})
	return service.wire(report), service.unavailable(cmp.Or(failure, problem))
}

func (service *OutboxRelayService) DeadEvents(authorization string) ([]DeadEvent, error) {
	failure := service.callers.Operator(authorization)
	documents, failure := service.deadUnless(failure)
	views, problem := service.deadEventsOf(documents)
	return views, service.unavailable(cmp.Or(failure, problem))
}

func (service *OutboxRelayService) RetryDead(authorization string, request RetryRequest) error {
	ref := outbox.Ref{Collection: request.Collection, Id: request.DocumentId}
	failure := service.callers.Operator(authorization)
	fields, failure := service.loadUnless(failure, ref)
	fields, failure = service.moveBackUnless(failure, fields, request)
	return service.saveUnless(service.missingLetter(failure), ref, fields)
}

func (service *OutboxRelayService) relay() (outbox.Relay, error) {
	settings, failure := service.settings.Outbox()
	return outbox.NewRelay(service.store, service.pusherOrDefault(), settings, time.Now), failure
}

func (service *OutboxRelayService) runUnless(failure error) (outbox.Report, error) {
	if failure != nil {
		return outbox.Report{}, failure
	}
	relay, problem := service.relay()
	report, runFailure := relay.RunOnce(service.sweepSize())
	return report, cmp.Or(problem, runFailure)
}

func (service *OutboxRelayService) deadUnless(failure error) ([]outbox.Document, error) {
	if failure != nil {
		return []outbox.Document{}, failure
	}
	return service.store.Dead(service.sweepSize())
}

func (service *OutboxRelayService) loadUnless(failure error, ref outbox.Ref) (outbox.Fields, error) {
	if failure != nil {
		return outbox.Fields{}, failure
	}
	return service.store.Load(ref)
}

func (service *OutboxRelayService) moveBackUnless(failure error, fields outbox.Fields, request RetryRequest) (outbox.Fields, error) {
	if failure != nil {
		return fields, failure
	}
	return outbox.DeadLetters{}.Requeue(fields, request.EventId, request.Subscriber)
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
	return outbox.NewHTTPPusher(service.tokens, outbox.HTTPPusher{}.DefaultTimeout())
}

func (service *OutboxRelayService) deadEventsOf(documents []outbox.Document) ([]DeadEvent, error) {
	views := []DeadEvent{}
	failures := []error{}
	for _, document := range documents {
		letters, failure := outbox.DeadLetters{}.List(document.Fields)
		views, failures = append(views, service.views(document.Ref, letters)...), append(failures, failure)
	}
	return views, cmp.Or(failures...)
}

func (service *OutboxRelayService) views(ref outbox.Ref, letters []outbox.DeadLetter) []DeadEvent {
	views := []DeadEvent{}
	for _, letter := range letters {
		views = append(views, DeadEvent{
			Collection: ref.Collection,
			DocumentId: ref.Id,
			EventId:    letter.Envelope.EventID,
			Type:       letter.Envelope.Type,
			Subscriber: letter.Subscriber,
			Attempts:   letter.Attempts,
			LastError:  letter.LastError,
			DeadAt:     letter.DeadAt,
		})
	}
	return views
}

func (service *OutboxRelayService) wire(report outbox.Report) OutboxReport {
	return OutboxReport{Documents: report.Documents, Delivered: report.Delivered, Retried: report.Retried, DeadLettered: report.DeadLettered}
}

func (service *OutboxRelayService) missingLetter(failure error) error {
	if errors.As(failure, &outbox.NotFound{}) {
		return &NotFound{What: "dead letter"}
	}
	return failure
}

func (service *OutboxRelayService) unavailable(failure error) error {
	switch {
	case failure == nil:
		return nil
	case errors.As(failure, new(*Unauthenticated)), errors.As(failure, new(*NotFound)):
		return failure
	}
	return &StorageUnavailable{Operation: "relay the outbox"}
}

func (service *OutboxRelayService) sweepSize() int {
	return 100
}
