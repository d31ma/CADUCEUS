package mailin

import (
	"errors"
	"time"

	"mesail.dev/packages/outbox"
)

type OutboxDesk struct {
	Store    OutboxStore
	Pusher   outbox.Pusher
	Settings SettingsReader
	Now      func() time.Time
}

func (desk OutboxDesk) RunOnce() (outbox.Report, error) {
	relay, err := desk.relay()
	if err != nil {
		return outbox.Report{}, err
	}
	return relay.RunOnce(desk.SweepSize())
}

func (desk OutboxDesk) Drain(ref outbox.Ref) {
	relay, err := desk.relay()
	if err == nil {
		relay.RelayDocument(ref)
	}
}

func (desk OutboxDesk) DeadEvents() ([]DeadEvent, error) {
	documents, err := desk.Store.Dead()
	if err != nil {
		return []DeadEvent{}, err
	}
	found := []DeadEvent{}
	failures := []error{}
	for _, document := range documents {
		letters, err := outbox.DeadLetters{}.List(document.Fields)
		found, failures = append(found, desk.views(document.Ref, letters)...), append(failures, err)
	}
	return found, errors.Join(failures...)
}

func (desk OutboxDesk) Retry(request RetryRequest) error {
	ref := outbox.Ref{Collection: request.Collection, Id: request.DocumentId}
	fields, err := desk.Store.Load(ref)
	if err != nil {
		return err
	}
	fields, err = outbox.DeadLetters{}.Requeue(fields, request.EventId, request.Subscriber)
	if err != nil {
		return desk.missingLetter(err)
	}
	return desk.Store.Save(ref, fields)
}

func (OutboxDesk) SweepSize() int {
	return 100
}

func (desk OutboxDesk) relay() (outbox.Relay, error) {
	settings, err := desk.Settings.Outbox()
	return outbox.NewRelay(desk.Store, desk.Pusher, settings, desk.Now), err
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

func (OutboxDesk) missingLetter(failure error) error {
	if errors.As(failure, &outbox.NotFound{}) {
		return Missing{Message: failure.Error()}
	}
	return Unavailable{Operation: "outbox document"}
}
