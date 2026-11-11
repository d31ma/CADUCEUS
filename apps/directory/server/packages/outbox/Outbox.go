package outbox

import "mesail.dev/packages/events"

type Outbox struct {
	settings Settings
}

func NewOutbox(settings Settings) Outbox {
	return Outbox{settings: settings}
}

func (outbox Outbox) Stage(fields Fields, envelope events.Envelope) (Fields, error) {
	state, err := Codec{}.Read(fields)
	if err != nil {
		return fields, err
	}
	event := PendingEvent{Envelope: envelope, Waiting: outbox.settings.consumersOf(envelope.Type)}
	state.Pending = outbox.withEvent(state.Pending, event)
	return Codec{}.Write(state)
}

func (Outbox) withEvent(pending []PendingEvent, event PendingEvent) []PendingEvent {
	if len(event.Waiting) == 0 {
		return pending
	}
	return append(pending, event)
}
