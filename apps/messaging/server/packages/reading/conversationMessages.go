package reading

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type conversationMessages struct {
	access core.Access
}

func (messages conversationMessages) in(caller oidc.Caller, conversation core.Conversation, messageId string) (core.Message, error) {
	message, _, err := messages.access.Message(caller, messageId)
	if err != nil {
		return core.Message{}, err
	}
	return message, messages.belonging(message, conversation)
}

func (conversationMessages) belonging(message core.Message, conversation core.Conversation) error {
	if message.ConversationId != (core.ConversationIds{}).Wire(conversation) {
		return refusals{}.noMessage()
	}
	return nil
}
