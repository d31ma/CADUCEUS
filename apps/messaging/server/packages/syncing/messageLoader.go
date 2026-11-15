package syncing

import "mesail.dev/packages/core"

type messageLoader struct {
	messages core.MessageStore
}

func (loader messageLoader) load(conversation core.Conversation, messageIds []string) ([]core.Message, error) {
	loaded := []core.Message{}
	for _, messageId := range messageIds {
		found, err := loader.one(conversation, messageId)
		if err != nil {
			return nil, err
		}
		loaded = append(loaded, found...)
	}
	return loaded, nil
}

func (loader messageLoader) one(conversation core.Conversation, messageId string) ([]core.Message, error) {
	month, storedId, err := core.MessageIds{}.Parse(messageId)
	if err != nil {
		return nil, nil
	}
	message, found, err := loader.messages.Find(conversation.DomainId, month, storedId)
	if err != nil {
		return nil, core.Unavailable{}.Store(err)
	}
	return loader.kept(message, found, core.ConversationIds{}.Wire(conversation)), nil
}

func (loader messageLoader) kept(message core.Message, found bool, conversationId string) []core.Message {
	if found && message.ConversationId == conversationId {
		return []core.Message{message}
	}
	return nil
}
