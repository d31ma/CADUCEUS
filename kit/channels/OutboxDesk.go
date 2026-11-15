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
	report, err := desk.relay().RunWithin(desk.sweepSize(), desk.timeBudget())
	return SweepReport{More: report.More}.Plus(report), err
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

func (desk OutboxDesk) relay() outbox.Relay {
	return outbox.NewRelay(desk.store, desk.pusher, desk.settings.Outbox, desk.now)
}

func (desk OutboxDesk) settingsError() error {
	if desk.settings.OutboxError != nil {
		return Unavailable{}.OutboxSettingsMissing(desk.settings.OutboxError)
	}
	return nil
}

func (OutboxDesk) letterError(failure error) error {
	missing := outbox.NotFound{}
	if errors.As(failure, &missing) {
		return NotFound{}.OutboxLetter(missing.Error(), missing.Number())
	}
	return Unavailable{}.OutboxUnreadable(failure)
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
	return 2 * time.Second
}
