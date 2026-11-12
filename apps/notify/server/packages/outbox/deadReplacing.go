package outbox

import "mesail.dev/packages/events"

type deadReplacing struct{}

func (replacing deadReplacing) added(dead []DeadLetter, letters []DeadLetter) ([]DeadLetter, error) {
	for _, letter := range letters {
		next, err := replacing.addedOne(dead, letter)
		if err != nil {
			return dead, err
		}
		dead = next
	}
	return dead, nil
}

func (deadReplacing) addedOne(dead []DeadLetter, letter DeadLetter) ([]DeadLetter, error) {
	key, err := events.ReplaceKeys{}.Of(letter.Envelope)
	if err != nil {
		return dead, err
	}
	kept, err := deadMatcher{key: deadKey{event: key, subscriber: letter.Subscriber}}.without(dead)
	return append(kept, letter), err
}
