package sending

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type MessageEvents struct {
	stager core.Stager
}

func NewMessageEvents(stager core.Stager) MessageEvents {
	return MessageEvents{stager: stager}
}

func (messageEvents MessageEvents) stage(message core.Message, conversation core.Conversation, readers []string) (core.Message, error) {
	finished := messageEvents.withHeader(message)
	rows, err := core.DeliveryCodec{}.Decode(finished.Deliveries)
	if err != nil {
		return message, err
	}
	fields, err := messageEvents.stager.Stage(finished.OutboxFields, events.Types{}.MessageCreated(), finished.DomainId, messageEvents.created(finished, conversation, readers))
	if err != nil {
		return message, err
	}
	fields, err = messageEvents.emails(fields, finished, DeliveryRows{}.email(rows))
	if err != nil {
		return message, err
	}
	finished.OutboxFields = fields
	finished.EventsStaged = true
	return finished, nil
}

func (messageEvents MessageEvents) withHeader(message core.Message) core.Message {
	if !message.IsEmail {
		return message
	}
	message.Email.MessageIdHeader = messageEvents.header(message)
	return message
}

func (messageEvents MessageEvents) emails(fields core.OutboxFields, message core.Message, emailRows []core.DeliveryRecord) (core.OutboxFields, error) {
	for _, row := range emailRows {
		staged, err := messageEvents.stager.Stage(fields, events.Types{}.EmailSendRequested(), message.DomainId, messageEvents.sendRequested(message, row))
		if err != nil {
			return fields, err
		}
		fields = staged
	}
	return fields, nil
}

func (messageEvents MessageEvents) created(message core.Message, conversation core.Conversation, readers []string) events.MessageCreated {
	lists := Lists{}
	return events.MessageCreated{
		MessageId:          messageEvents.wireId(message),
		ConversationId:     message.ConversationId,
		ConversationKind:   conversation.Kind,
		ThreadRootId:       message.ThreadRootId,
		AuthorKind:         message.AuthorKind,
		AuthorMemberId:     message.AuthorMemberId,
		AuthorAddress:      message.AuthorAddress,
		AuthorDisplayName:  message.AuthorDisplayName,
		Body:               message.Body,
		Subject:            message.Email.Subject,
		AttachmentIds:      lists.copied(message.AttachmentIds),
		FileNames:          messageEvents.fileNames(message),
		IsEmail:            message.IsEmail,
		MentionedMemberIds: lists.copied(message.MentionedMemberIds),
		MemberIds:          lists.copied(readers),
		CreatedAt:          message.CreatedAt,
	}
}

func (messageEvents MessageEvents) sendRequested(message core.Message, row core.DeliveryRecord) events.EmailSendRequested {
	lists := Lists{}
	wireId := messageEvents.wireId(message)
	return events.EmailSendRequested{
		MessageId:      wireId,
		From:           message.Email.From,
		Rcpt:           row.Recipient,
		Role:           row.Role,
		To:             lists.copied(message.Email.To),
		Cc:             lists.copied(message.Email.Cc),
		Subject:        message.Email.Subject,
		InReplyTo:      message.Email.InReplyTo,
		References:     lists.copied(message.Email.References),
		AttachmentIds:  lists.copied(message.AttachmentIds),
		IdempotencyKey: wireId + messageEvents.keySeparator() + row.Recipient,
	}
}

func (messageEvents MessageEvents) fileNames(message core.Message) []string {
	refs := core.AttachmentCodec{}.Refs(message)
	names := []string{}
	for _, ref := range refs {
		names = messageEvents.withName(names, ref.FileName)
	}
	return names
}

func (MessageEvents) withName(names []string, fileName string) []string {
	if fileName == "" {
		return names
	}
	return append(names, fileName)
}

func (messageEvents MessageEvents) header(message core.Message) string {
	return messageEvents.wireId(message) + "@" + message.DomainId
}

func (MessageEvents) wireId(message core.Message) string {
	return core.MessageIds{}.Wire(message.Month, message.Id)
}

func (MessageEvents) keySeparator() string {
	return "|"
}
