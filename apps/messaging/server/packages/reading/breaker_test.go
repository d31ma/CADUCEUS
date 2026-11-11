package reading_test

import (
	"errors"

	"mesail.dev/packages/core"
)

type breaker struct {
	broken map[string]bool
}

func (b *breaker) fail(name string) error {
	if b.broken[name] {
		return errors.New(name + " is down")
	}
	return nil
}

func (b *breaker) wrap(stores core.Stores) core.Stores {
	stores.Conversations = brokenConversations{ConversationStore: stores.Conversations, b: b}
	stores.Participants = brokenParticipants{ParticipantStore: stores.Participants, b: b}
	stores.Messages = brokenMessages{MessageStore: stores.Messages, b: b}
	stores.Reactions = brokenReactions{ReactionStore: stores.Reactions, b: b}
	stores.Mentions = brokenMentions{MentionStore: stores.Mentions, b: b}
	stores.AttachmentUses = brokenAttachmentUses{AttachmentUseStore: stores.AttachmentUses, b: b}
	return stores
}

type brokenConversations struct {
	core.ConversationStore
	b *breaker
}

func (s brokenConversations) Find(domainId string, storedPart string) (core.Conversation, bool, error) {
	if err := s.b.fail("conversations.find"); err != nil {
		return core.Conversation{}, false, err
	}
	return s.ConversationStore.Find(domainId, storedPart)
}

type brokenParticipants struct {
	core.ParticipantStore
	b *breaker
}

func (s brokenParticipants) OfMember(domainId string, memberId string) ([]core.Participant, error) {
	if err := s.b.fail("participants.ofMember"); err != nil {
		return nil, err
	}
	return s.ParticipantStore.OfMember(domainId, memberId)
}

func (s brokenParticipants) OfConversation(domainId string, conversationId string) ([]core.Participant, error) {
	if err := s.b.fail("participants.ofConversation"); err != nil {
		return nil, err
	}
	return s.ParticipantStore.OfConversation(domainId, conversationId)
}

func (s brokenParticipants) Save(participant core.Participant) error {
	if err := s.b.fail("participants.save"); err != nil {
		return err
	}
	return s.ParticipantStore.Save(participant)
}

type brokenMessages struct {
	core.MessageStore
	b *breaker
}

func (s brokenMessages) Find(domainId string, month string, id string) (core.Message, bool, error) {
	if err := s.b.fail("messages.find"); err != nil {
		return core.Message{}, false, err
	}
	return s.MessageStore.Find(domainId, month, id)
}

func (s brokenMessages) OfConversation(domainId string, month string, conversationId string) ([]core.Message, error) {
	if err := s.b.fail("messages.ofConversation"); err != nil {
		return nil, err
	}
	return s.MessageStore.OfConversation(domainId, month, conversationId)
}

func (s brokenMessages) Save(message core.Message) error {
	if err := s.b.fail("messages.save"); err != nil {
		return err
	}
	return s.MessageStore.Save(message)
}

type brokenReactions struct {
	core.ReactionStore
	b *breaker
}

func (s brokenReactions) ByKey(domainId string, key string) (core.Reaction, bool, error) {
	if err := s.b.fail("reactions.byKey"); err != nil {
		return core.Reaction{}, false, err
	}
	return s.ReactionStore.ByKey(domainId, key)
}

func (s brokenReactions) OfConversation(domainId string, conversationId string) ([]core.Reaction, error) {
	if err := s.b.fail("reactions.ofConversation"); err != nil {
		return nil, err
	}
	return s.ReactionStore.OfConversation(domainId, conversationId)
}

func (s brokenReactions) Save(reaction core.Reaction) error {
	if err := s.b.fail("reactions.save"); err != nil {
		return err
	}
	return s.ReactionStore.Save(reaction)
}

func (s brokenReactions) Insert(reaction core.Reaction) (string, error) {
	if err := s.b.fail("reactions.insert"); err != nil {
		return "", err
	}
	return s.ReactionStore.Insert(reaction)
}

type brokenMentions struct {
	core.MentionStore
	b *breaker
}

func (s brokenMentions) OfMember(domainId string, memberId string) ([]core.Mention, error) {
	if err := s.b.fail("mentions.ofMember"); err != nil {
		return nil, err
	}
	return s.MentionStore.OfMember(domainId, memberId)
}

type brokenAttachmentUses struct {
	core.AttachmentUseStore
	b *breaker
}

func (s brokenAttachmentUses) ByAttachment(domainId string, attachmentId string) ([]core.AttachmentUse, error) {
	if err := s.b.fail("attachmentUses.byAttachment"); err != nil {
		return nil, err
	}
	return s.AttachmentUseStore.ByAttachment(domainId, attachmentId)
}
