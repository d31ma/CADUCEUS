package outbox

import "mesail.dev/packages/events"

type staging struct{}

func (staging staging) added(pending []PendingEvent, event PendingEvent) ([]PendingEvent, error) {
	removed, err := events.ReplaceKeys{}.Removes(event.Envelope)
	if err != nil {
		return pending, err
	}
	replaced, err := events.ReplaceKeys{}.Of(event.Envelope)
	if err != nil {
		return pending, err
	}
	dropped, err := eventMatcher{key: removed}.split(pending)
	if err != nil {
		return pending, err
	}
	split, err := eventMatcher{key: replaced}.split(dropped.kept)
	if err != nil {
		return pending, err
	}
	event.Waiting = subscriberUnion{}.of(split.matchedWaiting(), event.Waiting)
	return staging.withEvent(split.kept, event), nil
}

func (staging) withEvent(pending []PendingEvent, event PendingEvent) []PendingEvent {
	if len(event.Waiting) == 0 {
		return pending
	}
	return append(pending, event)
}
