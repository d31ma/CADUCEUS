package outbox

import "mesail.dev/packages/events"

type Push struct {
	Subscriber string
	Envelope   events.Envelope
}
