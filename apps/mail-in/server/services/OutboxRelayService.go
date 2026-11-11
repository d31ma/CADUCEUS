import (
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
	pusher   outbox.Pusher
}

func (service *OutboxRelayService) RunOnce() (outbox.Report, error) {
	relay, err := service.relay()
	if err != nil {
		return outbox.Report{}, err
	}
	return relay.RunOnce(service.sweepSize())
}

func (service *OutboxRelayService) Drain(collection string, id string) {
	relay, err := service.relay()
	service.drainWith(relay, err, outbox.Ref{Collection: collection, Id: id})
}

func (service *OutboxRelayService) DeadEvents() ([]DeadEvent, error) {
	documents, err := service.store.Dead()
	if err != nil {
		return nil, err
	}
	return service.deadEventsOf(documents)
}

func (service *OutboxRelayService) Requeue(request RequeueRequest) error {
	ref := outbox.Ref{Collection: request.Collection, Id: request.DocumentId}
	fields, err := service.store.Load(ref)
	if err != nil {
		return err
	}
	fields, err = outbox.DeadLetters{}.Requeue(fields, request.EventId, request.Subscriber)
	if err != nil {
		return service.missingLetter(err)
	}
	return service.store.Save(ref, fields)
}

func (service *OutboxRelayService) relay() (outbox.Relay, error) {
	settings, err := service.settings.Outbox()
	return outbox.NewRelay(service.store, service.pusherOrDefault(), settings, time.Now), err
}

func (service *OutboxRelayService) drainWith(relay outbox.Relay, failure error, ref outbox.Ref) {
	if failure == nil {
		relay.RelayDocument(ref)
	}
}

func (service *OutboxRelayService) pusherOrDefault() outbox.Pusher {
	if service.pusher != nil {
		return service.pusher
	}
	return outbox.NewHTTPPusher(service.settings.ServiceToken(), outbox.HTTPPusher{}.DefaultTimeout())
}

func (service *OutboxRelayService) deadEventsOf(documents []outbox.Document) ([]DeadEvent, error) {
	found := []DeadEvent{}
	failures := []error{}
	for _, document := range documents {
		letters, err := outbox.DeadLetters{}.List(document.Fields)
		found, failures = append(found, service.views(document.Ref, letters)...), append(failures, err)
	}
	return found, errors.Join(failures...)
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

func (service *OutboxRelayService) missingLetter(failure error) error {
	if errors.As(failure, &outbox.NotFound{}) {
		return &NotFound{Message: failure.Error()}
	}
	return &StorageUnavailable{Operation: "outbox document"}
}

func (service *OutboxRelayService) sweepSize() int {
	return 100
}
