package channels

import (
	"errors"
	"time"

	"mesail.dev/packages/outbox"
)

type OutboxDesk struct {
	store    OutboxStore
	pusher   outbox.Pusher
	settings Settings
	now      func() time.Time
}

func NewOutboxDesk(documents Documents, pusher outbox.Pusher, settings Settings, now func() time.Time) OutboxDesk {
	return OutboxDesk{store: NewOutboxStore(documents), pusher: pusher, settings: settings, now: now}
}

func (desk OutboxDesk) RunOnce() (SweepReport, error) {
	err := desk.settingsError()
	if err != nil {
		return SweepReport{}, err
	}
	documents, err := desk.store.Pending(desk.sweepSize())
	if err != nil {
		return SweepReport{}, err
	}
	return desk.relayEach(documents)
}

func (desk OutboxDesk) Drain(collection string, id string) {
	desk.drainWhen(desk.settingsError() == nil, outbox.Ref{Collection: collection, Id: id})
}

func (desk OutboxDesk) Waiting() (int, error) {
	documents, err := desk.store.Pending(desk.waitingLimit())
	return len(documents), err
}

func (desk OutboxDesk) DeadEvents() ([]DeadEvent, error) {
	documents, err := desk.store.Dead()
	if err != nil {
		return nil, err
	}
	found := []DeadEvent{}
	failures := []error{}
	for _, document := range documents {
		letters, err := outbox.DeadLetters{}.List(document.Fields)
		found, failures = append(found, desk.views(document.Ref, letters)...), append(failures, err)
	}
	return found, errors.Join(failures...)
}

func (desk OutboxDesk) RetryDead(collection string, documentId string, eventId string, subscriber string) error {
	ref := outbox.Ref{Collection: collection, Id: documentId}
	fields, err := desk.store.Load(ref)
	if err != nil {
		return err
	}
	fields, err = outbox.DeadLetters{}.Requeue(fields, eventId, subscriber)
	if err != nil {
		return desk.letterError(err)
	}
	return desk.store.Save(ref, fields)
}

func (desk OutboxDesk) drainWhen(configured bool, ref outbox.Ref) {
	if configured {
		_, _ = desk.relay().RelayDocument(ref)
	}
}

func (desk OutboxDesk) relayEach(documents []outbox.Document) (SweepReport, error) {
	relay, started := desk.relay(), desk.now()
	report := SweepReport{}
	failures := []error{}
	index := 0
	for ; index < len(documents) && desk.now().Sub(started) < desk.timeBudget(); index++ {
		part, err := relay.RelayDocument(documents[index].Ref)
		report, failures = report.Plus(part), append(failures, err)
	}
	report.More = desk.stoppedEarly(index, len(documents))
	return report, errors.Join(failures...)
}

func (desk OutboxDesk) stoppedEarly(relayed int, found int) bool {
	return relayed < found || found == desk.sweepSize()
}

func (desk OutboxDesk) relay() outbox.Relay {
	return outbox.NewRelay(desk.store, desk.pusher, desk.settings.Outbox, desk.now)
}

func (desk OutboxDesk) settingsError() error {
	if desk.settings.OutboxError != nil {
		return &Unavailable{Message: "the outbox settings are missing", Cause: desk.settings.OutboxError}
	}
	return nil
}

func (OutboxDesk) letterError(failure error) error {
	if errors.As(failure, &outbox.NotFound{}) {
		return &NotFound{Message: failure.Error()}
	}
	return &Unavailable{Message: "the outbox document cannot be read", Cause: failure}
}

func (OutboxDesk) views(ref outbox.Ref, letters []outbox.DeadLetter) []DeadEvent {
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

func (OutboxDesk) sweepSize() int {
	return 25
}

func (OutboxDesk) waitingLimit() int {
	return 100
}

func (OutboxDesk) timeBudget() time.Duration {
	return 20 * time.Second
}
