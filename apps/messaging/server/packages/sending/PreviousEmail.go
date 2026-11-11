package sending

import (
	"slices"
	"time"

	"mesail.dev/packages/core"
)

type PreviousEmail struct {
	messages core.MessageStore
}

func NewPreviousEmail(messages core.MessageStore) PreviousEmail {
	return PreviousEmail{messages: messages}
}

func (previous PreviousEmail) find(domainId string, conversationId string, thread Thread, now time.Time) (core.Message, bool, error) {
	recent, err := previous.recent(domainId, conversationId, now)
	if err != nil {
		return core.Message{}, false, err
	}
	candidates := previous.eligible(slices.Concat(recent, previous.rootOf(thread)), thread.RootId)
	return previous.newest(candidates), len(candidates) > 0, nil
}

func (previous PreviousEmail) recent(domainId string, conversationId string, now time.Time) ([]core.Message, error) {
	found := []core.Message{}
	for _, month := range (Months{}).recent(now) {
		rows, err := previous.messages.OfConversation(domainId, month, conversationId)
		if err != nil {
			return nil, Failures{}.store(err)
		}
		found = append(found, rows...)
	}
	return found, nil
}

func (PreviousEmail) rootOf(thread Thread) []core.Message {
	if thread.RootId == "" {
		return nil
	}
	return []core.Message{thread.Root}
}

func (previous PreviousEmail) eligible(candidates []core.Message, rootId string) []core.Message {
	kept := []core.Message{}
	for _, message := range candidates {
		kept = append(kept, previous.ifEligible(message, previous.inThread(message, rootId))...)
	}
	return kept
}

func (PreviousEmail) inThread(message core.Message, rootId string) bool {
	return rootId == "" || message.ThreadRootId == rootId || core.MessageIds{}.Wire(message.Month, message.Id) == rootId
}

func (PreviousEmail) ifEligible(message core.Message, inThread bool) []core.Message {
	if message.Email.MessageIdHeader == "" || !inThread {
		return nil
	}
	return []core.Message{message}
}

func (previous PreviousEmail) newest(candidates []core.Message) core.Message {
	best := core.Message{}
	for _, message := range candidates {
		best = previous.newer(best, message)
	}
	return best
}

func (PreviousEmail) newer(best core.Message, message core.Message) core.Message {
	if message.Sequence > best.Sequence {
		return message
	}
	return best
}
