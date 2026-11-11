package reading

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type changeEvents struct{}

func (changeEvents) edited(message core.Message, memberIds []string) events.MessageEdited {
	return events.MessageEdited{
		MessageId:      core.MessageIds{}.Wire(message.Month, message.Id),
		ConversationId: message.ConversationId,
		Body:           message.Body,
		MemberIds:      memberIds,
		EditedAt:       message.EditedAt,
	}
}

func (changeEvents) deleted(message core.Message, memberIds []string) events.MessageDeleted {
	return events.MessageDeleted{
		MessageId:      core.MessageIds{}.Wire(message.Month, message.Id),
		ConversationId: message.ConversationId,
		MemberIds:      memberIds,
		DeletedAt:      message.DeletedAt,
	}
}
