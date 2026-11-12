package realtime

import "mesail.dev/packages/events"

type EventPayload struct {
	MessageId        string   `json:"messageId"`
	ConversationId   string   `json:"conversationId"`
	ConversationKind string   `json:"conversationKind"`
	ChannelId        string   `json:"channelId"`
	MemberId         string   `json:"memberId"`
	AuthorMemberId   string   `json:"authorMemberId"`
	CreatedBy        string   `json:"createdBy"`
	Emoji            string   `json:"emoji"`
	State            string   `json:"state"`
	MemberIds        []string `json:"memberIds"`
}

func (payload EventPayload) FromChannel() bool {
	return events.ReaderLists{}.FromChannel(payload.ConversationKind, payload.MemberIds)
}
