package sending

import (
	"time"

	"mesail.dev/packages/core"
)

type PositionWriter struct {
	stores core.Stores
	now    time.Time
}

func NewPositionWriter(stores core.Stores, now time.Time) PositionWriter {
	return PositionWriter{stores: stores, now: now}
}

func (writer PositionWriter) Write(draft Draft, message core.Message, mentionBudget int) error {
	err := writer.threadKeyRoute(message.IsEmail)(message)
	if err != nil {
		return err
	}
	err = writer.rootRoute(draft.Thread.RootId)(draft.Thread.Root)
	if err != nil {
		return err
	}
	err = writer.senderRoute(draft.Placement.Sender.Id)(writer.readPosition(draft.Placement.Sender, message))
	if err != nil {
		return err
	}
	return writer.mentions(message, message.MentionedMemberIds[:min(mentionBudget, len(message.MentionedMemberIds))])
}

func (writer PositionWriter) threadKeyRoute(isEmail bool) func(core.Message) error {
	if isEmail {
		return writer.insertThreadKey
	}
	return writer.skipThreadKey
}

func (PositionWriter) skipThreadKey(message core.Message) error {
	return nil
}

func (writer PositionWriter) insertThreadKey(message core.Message) error {
	threadKey := core.ThreadKey{
		DomainId:       message.DomainId,
		Key:            Hashes{}.threadKey(message.Email.MessageIdHeader),
		ConversationId: message.ConversationId,
		MessageId:      core.MessageIds{}.Wire(message.Month, message.Id),
	}
	_, err := writer.stores.ThreadKeys.Insert(threadKey)
	return writer.wrapped(err)
}

func (writer PositionWriter) rootRoute(rootId string) func(core.Message) error {
	if rootId == "" {
		return writer.skipRoot
	}
	return writer.saveRoot
}

func (PositionWriter) skipRoot(root core.Message) error {
	return nil
}

func (writer PositionWriter) saveRoot(root core.Message) error {
	root.ReplyCount++
	root.LastReplyAt = Months{}.stamp(writer.now)
	return writer.wrapped(writer.stores.Messages.Save(root))
}

func (PositionWriter) readPosition(sender core.Participant, message core.Message) core.Participant {
	sender.DomainId = message.DomainId
	sender.ConversationId = message.ConversationId
	sender.ReadSequence = message.Sequence
	sender.ReadMessageId = core.MessageIds{}.Wire(message.Month, message.Id)
	sender.ReadAt = message.CreatedAt
	return sender
}

func (writer PositionWriter) senderRoute(senderId string) func(core.Participant) error {
	if senderId == "" {
		return writer.insertSender
	}
	return writer.saveSender
}

func (writer PositionWriter) insertSender(sender core.Participant) error {
	_, err := writer.stores.Participants.Insert(sender)
	return writer.wrapped(err)
}

func (writer PositionWriter) saveSender(sender core.Participant) error {
	return writer.wrapped(writer.stores.Participants.Save(sender))
}

func (writer PositionWriter) mentions(message core.Message, memberIds []string) error {
	for _, memberId := range memberIds {
		mention := core.Mention{DomainId: message.DomainId, ConversationId: message.ConversationId, MemberId: memberId, MessageId: core.MessageIds{}.Wire(message.Month, message.Id), Sequence: message.Sequence}
		_, err := writer.stores.Mentions.Insert(mention)
		if err != nil {
			return Failures{}.store(err)
		}
	}
	return nil
}

func (PositionWriter) wrapped(err error) error {
	if err != nil {
		return Failures{}.store(err)
	}
	return nil
}
