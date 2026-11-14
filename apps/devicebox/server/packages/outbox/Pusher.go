package outbox

import "mesail.dev/packages/events"

type Pusher interface {
	Push(subscriber Subscriber, envelope events.Envelope) error
}
