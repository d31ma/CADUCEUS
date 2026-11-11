package inbound

import "mesail.dev/packages/core"

type ChannelMail struct {
	stores core.Stores
}

func NewChannelMail(stores core.Stores) ChannelMail {
	return ChannelMail{stores: stores}
}

func (channelMail ChannelMail) Place(email ReceivedEmail, channelId string) (Placement, error) {
	conversation, err := NewChannelConversations(channelMail.stores.Conversations).Existing(email.DomainId, channelId)
	if err != nil {
		return Placement{}, err
	}
	conversationId := core.ConversationIds{}.Wire(conversation)
	sender := EmailParticipants{}.Sender(email, conversationId)
	err = NewRoster(channelMail.stores.Participants).Ensure(email.DomainId, conversationId, []core.Participant{sender})
	if err != nil {
		return Placement{}, err
	}
	return Placement{Conversation: conversation}, nil
}
