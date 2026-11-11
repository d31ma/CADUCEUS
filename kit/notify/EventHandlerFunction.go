package notify

import "mesail.dev/packages/events"

type EventHandlerFunction func(envelope events.Envelope) error

func (handle EventHandlerFunction) Handle(envelope events.Envelope) error {
	return handle(envelope)
}
