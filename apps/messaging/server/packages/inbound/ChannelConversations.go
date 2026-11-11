package inbound

import "mesail.dev/packages/core"

type ChannelConversations struct {
	conversations core.ConversationStore
}

func NewChannelConversations(conversations core.ConversationStore) ChannelConversations {
	return ChannelConversations{conversations: conversations}
}

func (channels ChannelConversations) Existing(domainId string, channelId string) (core.Conversation, error) {
	conversation, found, err := channels.conversations.ByChannel(domainId, channelId)
	if err != nil {
		return core.Conversation{}, Unavailability{}.Of(err)
	}
	return conversation, channels.required(found)
}

func (ChannelConversations) required(found bool) error {
	if found {
		return nil
	}
	return Unavailability{}.Missing("the channel conversation")
}
