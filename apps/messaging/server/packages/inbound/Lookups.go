package inbound

import "mesail.dev/packages/core"

type Lookups struct {
	stores core.Stores
}

func NewLookups(stores core.Stores) Lookups {
	return Lookups{stores: stores}
}

func (lookups Lookups) ExistingConversation(domainId string, wireId string) (core.Conversation, error) {
	storedPart, err := core.ConversationIds{}.Parse(wireId)
	if err != nil {
		return core.Conversation{}, err
	}
	conversation, found, err := lookups.stores.Conversations.Find(domainId, storedPart)
	if err != nil {
		return core.Conversation{}, Unavailability{}.Of(err)
	}
	return conversation, lookups.required(found, "the conversation")
}

func (lookups Lookups) ExistingMessage(domainId string, wireId string) (core.Message, error) {
	month, storedId, err := core.MessageIds{}.Parse(wireId)
	if err != nil {
		return core.Message{}, err
	}
	message, found, err := lookups.stores.Messages.Find(domainId, month, storedId)
	if err != nil {
		return core.Message{}, Unavailability{}.Of(err)
	}
	return message, lookups.required(found, "the message")
}

func (Lookups) required(found bool, what string) error {
	if found {
		return nil
	}
	return Unavailability{}.Missing(what)
}
