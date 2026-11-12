package outbox

import "mesail.dev/packages/events"

type deadKey struct {
	event      events.EventKey
	subscriber string
}
