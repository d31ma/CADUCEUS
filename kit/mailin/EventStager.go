package mailin

import (
	"io"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

type EventStager struct {
	Settings SettingsReader
	Now      func() time.Time
	Random   io.Reader
}

func (stager EventStager) Stage(stored InboundRaw, envelope Envelope) (EventPatch, error) {
	settings, err := stager.Settings.Outbox()
	if err != nil {
		return EventPatch{}, err
	}
	payload := Payloads{}.EmailReceived(stored, envelope)
	staged, err := events.NewEnvelopeFactory(stager.Producer(), stager.Now, stager.Random).New(events.Types{}.EmailReceived(), stored.DomainId, payload)
	if err != nil {
		return EventPatch{}, err
	}
	fields, err := outbox.NewOutbox(settings).Stage(stored.OutboxFields(), staged)
	return stager.patch(fields), err
}

func (EventStager) Producer() string {
	return "mail-in"
}

func (EventStager) patch(fields outbox.Fields) EventPatch {
	outboxPatch := OutboxPatch{}.From(fields)
	return EventPatch{
		PendingEvents: outboxPatch.PendingEvents,
		DeadEvents:    outboxPatch.DeadEvents,
		OutboxPending: outboxPatch.OutboxPending,
		OutboxDead:    outboxPatch.OutboxDead,
		OutboxWaiting: outboxPatch.OutboxWaiting,
		OutboxHasDead: outboxPatch.OutboxHasDead,
		EventQueued:   true,
	}
}
