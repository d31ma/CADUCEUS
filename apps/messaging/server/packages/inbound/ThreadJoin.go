package inbound

import "mesail.dev/packages/core"

type ThreadJoin struct {
	stores core.Stores
	ports  core.Ports
}

func NewThreadJoin(stores core.Stores, ports core.Ports) ThreadJoin {
	return ThreadJoin{stores: stores, ports: ports}
}

func (join ThreadJoin) Join(email ReceivedEmail, key core.ThreadKey) (Placement, error) {
	lookups := NewLookups(join.stores)
	conversation, err := lookups.ExistingConversation(email.DomainId, key.ConversationId)
	if err != nil {
		return Placement{}, err
	}
	matched, err := lookups.ExistingMessage(email.DomainId, key.MessageId)
	if err != nil {
		return Placement{}, err
	}
	root, err := join.root(email.DomainId, matched)
	if err != nil {
		return Placement{}, err
	}
	conversationId := core.ConversationIds{}.Wire(conversation)
	members, err := join.members(email, conversation)
	if err != nil {
		return Placement{}, err
	}
	wanted := append([]core.Participant{EmailParticipants{}.Sender(email, conversationId)}, members...)
	err = NewRoster(join.stores.Participants).Ensure(email.DomainId, conversationId, wanted)
	if err != nil {
		return Placement{}, err
	}
	return Placement{Conversation: conversation, Root: root, HasRoot: true}, nil
}

func (join ThreadJoin) root(domainId string, matched core.Message) (core.Message, error) {
	if matched.ThreadRootId == "" {
		return matched, nil
	}
	return NewLookups(join.stores).ExistingMessage(domainId, matched.ThreadRootId)
}

func (join ThreadJoin) members(email ReceivedEmail, conversation core.Conversation) ([]core.Participant, error) {
	if conversation.Kind != (core.ConversationKinds{}).Direct() {
		return []core.Participant{}, nil
	}
	return NewRecipientDirectory(join.stores, join.ports).Members(email, core.ConversationIds{}.Wire(conversation))
}
