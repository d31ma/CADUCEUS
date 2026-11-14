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
	state.Pending, err = staging{}.added(state.Pending, event)
	if err != nil {
		return fields, err
	}
	staged, err := Codec{}.Write(state)
	if err != nil {
		return fields, err
	}
	return documentCap{}.checked(fields, staged)
}
