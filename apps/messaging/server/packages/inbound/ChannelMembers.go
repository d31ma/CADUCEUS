package inbound

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type ChannelMembers struct {
	stores core.Stores
}

func NewChannelMembers(stores core.Stores) ChannelMembers {
	return ChannelMembers{stores: stores}
}

func (channelMembers ChannelMembers) Added(envelope events.Envelope) error {
	payload := events.ChannelMemberAdded{}
	err := events.PayloadReader{}.Read(envelope, &payload)
	if err != nil {
		return err
	}
	conversation, err := NewChannelConversations(channelMembers.stores.Conversations).Existing(envelope.DomainId, payload.ChannelId)
	if err != nil {
		return err
	}
	member, _, err := channelMembers.stores.Members.ByMemberId(envelope.DomainId, payload.MemberId)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	conversationId := core.ConversationIds{}.Wire(conversation)
	participant := channelMembers.participant(envelope.DomainId, conversationId, payload.MemberId, member)
	return NewRoster(channelMembers.stores.Participants).Ensure(envelope.DomainId, conversationId, []core.Participant{participant})
}

func (channelMembers ChannelMembers) Removed(envelope events.Envelope) error {
	payload := events.ChannelMemberRemoved{}
	err := events.PayloadReader{}.Read(envelope, &payload)
	if err != nil {
		return err
	}
	participants, err := channelMembers.stores.Participants.OfConversation(envelope.DomainId, payload.ConversationId)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	err = channelMembers.delete(envelope.DomainId, Participants{}.OfMember(participants, payload.MemberId))
	if err != nil {
		return Unavailability{}.Of(err)
	}
	return nil
}

func (channelMembers ChannelMembers) delete(domainId string, participants []core.Participant) error {
	for _, participant := range participants {
		err := channelMembers.stores.Participants.Delete(domainId, participant.Id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ChannelMembers) participant(domainId string, conversationId string, memberId string, member core.Member) core.Participant {
	return core.Participant{
		DomainId:       domainId,
		ConversationId: conversationId,
		Kind:           core.ParticipantKinds{}.Member(),
		MemberId:       memberId,
		Address:        Addresses{}.Normal(member.PrimaryAddress),
		DisplayName:    member.DisplayName,
		Role:           core.RecipientRoles{}.Member(),
	}
}
