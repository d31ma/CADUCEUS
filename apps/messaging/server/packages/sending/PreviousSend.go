package sending

import (
	"time"

	"mesail.dev/packages/core"
)

type PreviousSend struct {
	messages core.MessageStore
}

func NewPreviousSend(messages core.MessageStore) PreviousSend {
	return PreviousSend{messages: messages}
}

func (previous PreviousSend) find(domainId string, sendKey string, now time.Time) (core.Message, bool, error) {
	matches := []core.Message{}
	for _, month := range (Months{}).recent(now) {
		message, found, err := previous.messages.BySendKey(domainId, month, sendKey)
		if err != nil {
			return core.Message{}, false, Failures{}.store(err)
		}
		matches = append(matches, previous.ifFound(message, found)...)
	}
	return previous.first(matches)
}

func (PreviousSend) ifFound(message core.Message, found bool) []core.Message {
	if found {
		return []core.Message{message}
	}
	return nil
}

func (PreviousSend) first(matches []core.Message) (core.Message, bool, error) {
	if len(matches) == 0 {
		return core.Message{}, false, nil
	}
	return matches[0], true, nil
}
