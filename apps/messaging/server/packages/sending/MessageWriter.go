package sending

import (
	"encoding/json"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type MessageWriter struct {
	stores    core.Stores
	events    MessageEvents
	uses      AttachmentUses
	positions PositionWriter
	now       time.Time
}

func NewMessageWriter(stores core.Stores, stager core.Stager, now time.Time) MessageWriter {
	return MessageWriter{stores: stores, events: NewMessageEvents(stager), uses: NewAttachmentUses(stores.AttachmentUses), positions: NewPositionWriter(stores, now), now: now}
}

func (writer MessageWriter) Write(caller oidc.Caller, draft Draft, mentionBudget int) (core.Message, error) {
	conversation, err := writer.conversation(draft.Placement.Conversation)
	if err != nil {
		return core.Message{}, err
	}
	err = writer.participants(caller.DomainId, core.ConversationIds{}.Wire(conversation), draft.Placement.Newcomers)
	if err != nil {
		return core.Message{}, err
	}
	inserted, err := writer.insert(writer.message(caller, draft, conversation))
	if err != nil {
		return core.Message{}, err
	}
	err = writer.uses.write(inserted)
	if err != nil {
		return core.Message{}, err
	}
	staged, err := writer.events.stage(inserted, conversation, Readers{}.of(draft.Placement.all()))
	if err != nil {
		return core.Message{}, err
	}
	err = writer.save(staged)
	if err != nil {
		return core.Message{}, err
	}
	return staged, writer.positions.Write(draft, staged, mentionBudget)
}

func (writer MessageWriter) conversation(conversation core.Conversation) (core.Conversation, error) {
	conversation.MessageCount++
	conversation.LastMessageAt = Months{}.stamp(writer.now)
	return writer.conversationRoute(conversation.Id)(conversation)
}

func (writer MessageWriter) conversationRoute(id string) func(core.Conversation) (core.Conversation, error) {
	if id == "" {
		return writer.insertConversation
	}
	return writer.saveConversation
}

func (writer MessageWriter) insertConversation(conversation core.Conversation) (core.Conversation, error) {
	id, err := writer.stores.Conversations.Insert(conversation)
	if err != nil {
		return conversation, Failures{}.store(err)
	}
	conversation.Id = id
	return conversation, nil
}

func (writer MessageWriter) saveConversation(conversation core.Conversation) (core.Conversation, error) {
	err := writer.stores.Conversations.Save(conversation)
	if err != nil {
		return conversation, Failures{}.store(err)
	}
	return conversation, nil
}

func (writer MessageWriter) participants(domainId string, conversationId string, newcomers []core.Participant) error {
	for _, participant := range newcomers {
		participant.DomainId = domainId
		participant.ConversationId = conversationId
		_, err := writer.stores.Participants.Insert(participant)
		if err != nil {
			return Failures{}.store(err)
		}
	}
	return nil
}

func (writer MessageWriter) message(caller oidc.Caller, draft Draft, conversation core.Conversation) core.Message {
	lists := Lists{}
	blocks, _ := json.Marshal(append([]core.Block{}, draft.Rich.Blocks...))
	return core.Message{
		DomainId:           caller.DomainId,
		ConversationId:     core.ConversationIds{}.Wire(conversation),
		ThreadRootId:       draft.Thread.RootId,
		AlsoToConversation: draft.Request.AlsoToConversation,
		Month:              core.MessageIds{}.Month(writer.now),
		Sequence:           conversation.MessageCount,
		AuthorKind:         core.ParticipantKinds{}.Member(),
		AuthorMemberId:     caller.MemberId,
		AuthorAddress:      lists.address(caller.Email),
		AuthorDisplayName:  caller.DisplayName,
		ClientMessageId:    draft.Request.ClientMessageId,
		SendKey:            draft.SendKey,
		Body:               draft.Request.Body,
		BodyFormat:         core.BodyFormats{}.Markdown(),
		BlocksJson:         string(blocks),
		Truncated:          draft.Rich.Truncated,
		AttachmentIds:      lists.copied(draft.Request.AttachmentIds),
		MentionedMemberIds: lists.copied(draft.Mentions),
		IsEmail:            draft.IsEmail,
		Email:              draft.Email,
		Deliveries:         core.DeliveryCodec{}.Encode(draft.Rows),
		CreatedAt:          Months{}.stamp(writer.now),
	}
}

func (writer MessageWriter) insert(message core.Message) (core.Message, error) {
	id, err := writer.stores.Messages.Insert(message)
	if err != nil {
		return message, Failures{}.store(err)
	}
	message.Id = id
	return message, nil
}

func (writer MessageWriter) save(message core.Message) error {
	err := writer.stores.Messages.Save(message)
	if err != nil {
		return Failures{}.store(err)
	}
	return nil
}
