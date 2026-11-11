package inbound

import "mesail.dev/packages/core"

type DirectMail struct {
	stores core.Stores
	ports  core.Ports
}

func NewDirectMail(stores core.Stores, ports core.Ports) DirectMail {
	return DirectMail{stores: stores, ports: ports}
}

func (direct DirectMail) Place(email ReceivedEmail, entries []core.AddressEntry) (Placement, error) {
	conversation, err := direct.conversation(email)
	if err != nil {
		return Placement{}, err
	}
	conversationId := core.ConversationIds{}.Wire(conversation)
	members, err := NewRecipientDirectory(direct.stores, direct.ports).MemberParticipants(email.DomainId, conversationId, entries)
	if err != nil {
		return Placement{}, err
	}
	wanted := EmailParticipants{}.Direct(email, conversationId, members)
	err = NewRoster(direct.stores.Participants).Ensure(email.DomainId, conversationId, wanted)
	if err != nil {
		return Placement{}, err
	}
	return Placement{Conversation: conversation}, nil
}

func (direct DirectMail) conversation(email ReceivedEmail) (core.Conversation, error) {
	existing, found, err := direct.stores.Conversations.ByParticipantKey(email.DomainId, email.SendKey)
	if err != nil {
		return core.Conversation{}, Unavailability{}.Of(err)
	}
	conversation, err := direct.insertUnlessFound(direct.newConversation(email), existing, found)
	if err != nil {
		return core.Conversation{}, Unavailability{}.Of(err)
	}
	return conversation, nil
}

func (direct DirectMail) insertUnlessFound(conversation core.Conversation, existing core.Conversation, found bool) (core.Conversation, error) {
	if found {
		return existing, nil
	}
	id, err := direct.stores.Conversations.Insert(conversation)
	conversation.Id = id
	return conversation, err
}

func (direct DirectMail) newConversation(email ReceivedEmail) core.Conversation {
	now := direct.ports.Now()
	return core.Conversation{
		DomainId:       email.DomainId,
		Kind:           core.ConversationKinds{}.Direct(),
		Subject:        email.Mail.Headers.Subject,
		HasExternal:    true,
		ParticipantKey: email.SendKey,
		CreatedMonth:   core.MessageIds{}.Month(now),
		CreatedAt:      Timestamps{}.Of(now),
	}
}
