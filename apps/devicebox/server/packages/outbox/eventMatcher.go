package outbox

import "mesail.dev/packages/events"

type eventMatcher struct {
	key events.EventKey
}

func (matcher eventMatcher) split(pending []PendingEvent) (pendingSplit, error) {
	split := pendingSplit{kept: []PendingEvent{}, matched: []PendingEvent{}}
	for _, event := range pending {
		found, err := matcher.matches(event.Envelope)
		if err != nil {
			return split, err
		}
		split = split.with(event, found)
	}
	return split, nil
}

func (matcher eventMatcher) matches(envelope events.Envelope) (bool, error) {
	if envelope.Type != matcher.key.Type {
		return false, nil
	}
	key, err := events.ReplaceKeys{}.Of(envelope)
	return key == matcher.key, err
}
