package events

type ReaderLists struct{}

func (ReaderLists) MostDirectMembers() int {
	return 100
}

func (ReaderLists) MostMentions() int {
	return 100
}

func (ReaderLists) ChannelKind() string {
	return "channel"
}

func (ReaderLists) DirectKind() string {
	return "direct"
}

func (lists ReaderLists) ForConversation(conversationKind string, memberIds []string) []string {
	return lists.keptFor(conversationKind)(memberIds)
}

func (lists ReaderLists) Mentions(memberIds []string) []string {
	return lists.copied(memberIds[:min(len(memberIds), lists.MostMentions())])
}

func (lists ReaderLists) FromChannel(conversationKind string, memberIds []string) bool {
	return lists.fromChannel(conversationKind, memberIds)
}

func (lists ReaderLists) keptFor(conversationKind string) func([]string) []string {
	if conversationKind == lists.ChannelKind() {
		return lists.none
	}
	return lists.copied
}

func (lists ReaderLists) fromChannel(conversationKind string, memberIds []string) bool {
	return conversationKind == lists.ChannelKind() || len(memberIds) == 0
}

func (ReaderLists) none(memberIds []string) []string {
	return []string{}
}

func (ReaderLists) copied(memberIds []string) []string {
	return append([]string{}, memberIds...)
}
