package reading

import "mesail.dev/packages/core"

type messageFilter struct {
	threadRootId string
	before       int
}

func (filter messageFilter) matching(messages []core.Message) []core.Message {
	kept := []core.Message{}
	for _, message := range messages {
		kept = append(kept, filter.ifMatching(message)...)
	}
	return kept
}

func (filter messageFilter) ifMatching(message core.Message) []core.Message {
	if filter.listed(message) && message.Sequence < filter.before {
		return []core.Message{message}
	}
	return nil
}

func (filter messageFilter) listed(message core.Message) bool {
	return message.ThreadRootId == filter.threadRootId || (filter.threadRootId == "" && message.AlsoToConversation)
}
