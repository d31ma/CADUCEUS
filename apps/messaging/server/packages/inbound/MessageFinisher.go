package inbound

import (
	"strings"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

type MessageFinisher struct {
	stores core.Stores
	ports  core.Ports
}

func NewMessageFinisher(stores core.Stores, ports core.Ports) MessageFinisher {
	return MessageFinisher{stores: stores, ports: ports}
}

func (finisher MessageFinisher) Finish(placement Placement, message core.Message) error {
	conversation := finisher.counted(placement.Conversation, message)
	err := finisher.stores.Conversations.Save(conversation)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	err = finisher.ensureThreadKey(message)
	if err != nil {
		return err
	}
	err = finisher.replyToRoot(placement, message)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	return finisher.publish(conversation, message)
}

func (finisher MessageFinisher) publish(conversation core.Conversation, message core.Message) error {
	participants, err := finisher.stores.Participants.OfConversation(message.DomainId, message.ConversationId)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	payload := finisher.payload(conversation, message, Participants{}.MemberIds(participants))
	message.OutboxFields, err = finisher.ports.Stager.Stage(message.OutboxFields, events.Types{}.MessageCreated(), message.DomainId, payload)
	if err != nil {
		return err
	}
	message.EventsStaged = true
	err = finisher.stores.Messages.Save(message)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	return nil
}

func (MessageFinisher) counted(conversation core.Conversation, message core.Message) core.Conversation {
	conversation.MessageCount = max(conversation.MessageCount, message.Sequence)
	conversation.LastMessageAt = max(conversation.LastMessageAt, message.CreatedAt)
	return conversation
}

func (finisher MessageFinisher) ensureThreadKey(message core.Message) error {
	key := finisher.threadKey(message)
	_, found, err := finisher.stores.ThreadKeys.ByKey(message.DomainId, key.Key)
	if err != nil {
		return Unavailability{}.Of(err)
	}
	err = finisher.insertUnless(key, finisher.skipThreadKey(found, message.Email.MessageIdHeader))
	if err != nil {
		return Unavailability{}.Of(err)
	}
	return nil
}

func (MessageFinisher) threadKey(message core.Message) core.ThreadKey {
	return core.ThreadKey{
		DomainId:       message.DomainId,
		Key:            MessageIdKeys{}.Of(message.Email.MessageIdHeader),
		ConversationId: message.ConversationId,
		MessageId:      core.MessageIds{}.Wire(message.Month, message.Id),
	}
}

func (MessageFinisher) skipThreadKey(found bool, messageIdHeader string) bool {
	return found || strings.TrimSpace(messageIdHeader) == ""
}

func (finisher MessageFinisher) insertUnless(key core.ThreadKey, skip bool) error {
	if skip {
		return nil
	}
	_, err := finisher.stores.ThreadKeys.Insert(key)
	return err
}

func (finisher MessageFinisher) replyToRoot(placement Placement, message core.Message) error {
	if !placement.HasRoot {
		return nil
	}
	root := placement.Root
	root.ReplyCount++
	root.LastReplyAt = message.CreatedAt
	return finisher.stores.Messages.Save(root)
}

func (MessageFinisher) payload(conversation core.Conversation, message core.Message, memberIds []string) events.MessageCreated {
	return events.MessageCreated{
		MessageId:          core.MessageIds{}.Wire(message.Month, message.Id),
		ConversationId:     message.ConversationId,
		ConversationKind:   conversation.Kind,
		ThreadRootId:       message.ThreadRootId,
		AuthorKind:         message.AuthorKind,
		AuthorMemberId:     message.AuthorMemberId,
		AuthorAddress:      message.AuthorAddress,
		AuthorDisplayName:  message.AuthorDisplayName,
		Body:               message.Body,
		Subject:            message.Email.Subject,
		AttachmentIds:      append([]string{}, message.AttachmentIds...),
		FileNames:          []string{},
		IsEmail:            message.IsEmail,
		MentionedMemberIds: append([]string{}, message.MentionedMemberIds...),
		MemberIds:          memberIds,
		CreatedAt:          message.CreatedAt,
	}
}
