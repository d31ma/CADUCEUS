package outbox

import "mesail.dev/packages/events"

type deadMatcher struct {
	key deadKey
}

func (matcher deadMatcher) without(dead []DeadLetter) ([]DeadLetter, error) {
	kept := []DeadLetter{}
	for _, letter := range dead {
		found, err := matcher.matches(letter)
		if err != nil {
			return dead, err
		}
		kept = matcher.keptUnless(kept, letter, found)
	}
	return kept, nil
}

func (deadMatcher) keptUnless(kept []DeadLetter, letter DeadLetter, found bool) []DeadLetter {
	if found {
		return kept
	}
	return append(kept, letter)
}

func (matcher deadMatcher) matches(letter DeadLetter) (bool, error) {
	if letter.Envelope.Type != matcher.key.event.Type {
		return false, nil
	}
	key, err := events.ReplaceKeys{}.Of(letter.Envelope)
	return deadKey{event: key, subscriber: letter.Subscriber} == matcher.key, err
}
