package outbox

import "mesail.dev/packages/events"

type PushFunc func(subscriber Subscriber, envelope events.Envelope) error

func (push PushFunc) Push(subscriber Subscriber, envelope events.Envelope) error {
	return push(subscriber, envelope)
}
