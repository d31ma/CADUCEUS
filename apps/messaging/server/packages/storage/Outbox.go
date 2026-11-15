package storage

import (
	"cmp"
	"errors"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/outbox"
)

type Outbox struct {
	Store           OutboxStore
	Pusher          outbox.Pusher
	Settings        outbox.Settings
	SettingsFailure error
	Now             func() time.Time
}

func (relay Outbox) RunOnce(limit int) (outbox.Report, error) {
	if relay.SettingsFailure != nil {
		return outbox.Report{}, relay.SettingsFailure
	}
	return relay.relay().RunOnce(limit)
}

func (relay Outbox) Drain(ref outbox.Ref) {
	if relay.SettingsFailure != nil {
		return
	}
	relay.relay().RelayDocument(ref)
}

func (relay Outbox) Dead() ([]outbox.Document, error) {
	return relay.Store.Dead()
}

func (relay Outbox) Retry(ref outbox.Ref, eventId string, subscriber string) error {
	fields, failure := relay.Store.Load(ref)
	requeued, requeueFailure := outbox.DeadLetters{}.Requeue(fields, eventId, subscriber)
	return relay.saveUnless(cmp.Or(failure, relay.missingLetter(requeueFailure)), ref, requeued)
}

func (relay Outbox) relay() outbox.Relay {
	return outbox.NewRelay(relay.Store, relay.Pusher, relay.Settings, relay.Now)
}

func (relay Outbox) saveUnless(failure error, ref outbox.Ref, fields outbox.Fields) error {
	if failure != nil {
		return failure
	}
	return relay.Store.Save(ref, fields)
}

func (relay Outbox) missingLetter(failure error) error {
	return cmp.Or(relay.kitMissing(failure), relay.unreadableLetters(failure))
}

func (Outbox) kitMissing(failure error) error {
	missing := outbox.NotFound{}
	if !errors.As(failure, &missing) {
		return nil
	}
	return core.NotFound{}.Kit(missing.Error(), missing.Number())
}

func (Outbox) unreadableLetters(failure error) error {
	if failure != nil {
		return core.NotFound{}.NoDeadEvent()
	}
	return nil
}
