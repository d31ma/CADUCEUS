package core

import "mesail.dev/packages/oidc"

type Access struct {
	stores Stores
}

func NewAccess(stores Stores) Access {
	return Access{stores: stores}
}

func (access Access) Conversation(caller oidc.Caller, conversationWireId string) (Conversation, error) {
	conversation, err := access.stored(caller, conversationWireId)
	if err != nil {
		return Conversation{}, err
	}
	_, joined, err := access.Participant(caller, conversation)
	if err != nil {
		return Conversation{}, err
	}
	return conversation, access.readRefusal(joined || access.public(conversation))
}

func (access Access) Postable(caller oidc.Caller, conversationWireId string) (Conversation, Participant, error) {
	conversation, err := access.stored(caller, conversationWireId)
	if err != nil {
		return Conversation{}, Participant{}, err
	}
	participant, joined, err := access.Participant(caller, conversation)
	if err != nil {
		return Conversation{}, Participant{}, err
	}
	return conversation, participant, access.postRefusal(joined, conversation.Archived)
}

func (access Access) Message(caller oidc.Caller, messageWireId string) (Message, Conversation, error) {
	month, storedId, err := MessageIds{}.Parse(messageWireId)
	if err != nil {
		return Message{}, Conversation{}, err
	}
	message, found, err := access.stores.Messages.Find(caller.DomainId, month, storedId)
	if err != nil {
		return Message{}, Conversation{}, access.unavailable(err)
	}
	err = access.messageRefusal(found && message.DomainId == caller.DomainId)
	if err != nil {
		return Message{}, Conversation{}, err
	}
	conversation, err := access.Conversation(caller, message.ConversationId)
	return message, conversation, err
}

func (access Access) Participant(caller oidc.Caller, conversation Conversation) (Participant, bool, error) {
	participations, err := access.stores.Participants.OfMember(caller.DomainId, caller.MemberId)
	if err != nil {
		return Participant{}, false, access.unavailable(err)
	}
	return access.matching(participations, ConversationIds{}.Wire(conversation))
}

func (access Access) stored(caller oidc.Caller, conversationWireId string) (Conversation, error) {
	storedPart, err := ConversationIds{}.Parse(conversationWireId)
	if err != nil {
		return Conversation{}, err
	}
	conversation, found, err := access.stores.Conversations.Find(caller.DomainId, storedPart)
	if err != nil {
		return Conversation{}, access.unavailable(err)
	}
	return conversation, access.readRefusal(found && conversation.DomainId == caller.DomainId)
}

func (access Access) matching(participations []Participant, conversationId string) (Participant, bool, error) {
	for _, participant := range participations {
		if participant.ConversationId == conversationId {
			return participant, true, nil
		}
	}
	return Participant{}, false, nil
}

func (access Access) public(conversation Conversation) bool {
	return conversation.Kind == ConversationKinds{}.Channel() && conversation.Visibility == Visibilities{}.Public()
}

func (access Access) readRefusal(allowed bool) error {
	if allowed {
		return nil
	}
	return &NotFound{Message: "no such conversation"}
}

func (access Access) messageRefusal(allowed bool) error {
	if allowed {
		return nil
	}
	return &NotFound{Message: "no such message"}
}

func (access Access) postRefusal(joined bool, archived bool) error {
	if !joined {
		return &NotFound{Message: "no such conversation"}
	}
	return access.archivedRefusal(archived)
}

func (access Access) archivedRefusal(archived bool) error {
	if archived {
		return &Conflict{Message: "the channel is archived and read-only"}
	}
	return nil
}

func (access Access) unavailable(failure error) error {
	return &Unavailable{Message: "the message store is not available", Cause: failure}
}
