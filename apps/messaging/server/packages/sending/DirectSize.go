package sending

import (
	"strconv"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type DirectSize struct{}

func (size DirectSize) check(placement Placement) error {
	return size.refusal(placement.Conversation.Kind, len(Readers{}.of(placement.all())))
}

func (size DirectSize) refusal(conversationKind string, memberCount int) error {
	if conversationKind != (core.ConversationKinds{}).Direct() || memberCount <= size.most() {
		return nil
	}
	return Failures{}.recipients("a direct conversation has at most " + strconv.Itoa(size.most()) + " member participants")
}

func (DirectSize) most() int {
	return events.ReaderLists{}.MostDirectMembers()
}
