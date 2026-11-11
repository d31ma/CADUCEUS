package reading

import "mesail.dev/packages/core"

type storage struct {
	stores core.Stores
}

func (storage storage) participationsOf(domainId string, memberId string) ([]core.Participant, error) {
	rows, err := storage.stores.Participants.OfMember(domainId, memberId)
	return rows, storage.wrapped(err)
}

func (storage storage) participantsIn(domainId string, conversationId string) ([]core.Participant, error) {
	rows, err := storage.stores.Participants.OfConversation(domainId, conversationId)
	return rows, storage.wrapped(err)
}

func (storage storage) mentionsOf(domainId string, memberId string) ([]core.Mention, error) {
	rows, err := storage.stores.Mentions.OfMember(domainId, memberId)
	return rows, storage.wrapped(err)
}

func (storage storage) conversation(domainId string, storedPart string) (core.Conversation, bool, error) {
	conversation, found, err := storage.stores.Conversations.Find(domainId, storedPart)
	return conversation, found, storage.wrapped(err)
}

func (storage storage) message(domainId string, month string, storedId string) (core.Message, bool, error) {
	message, found, err := storage.stores.Messages.Find(domainId, month, storedId)
	return message, found, storage.wrapped(err)
}

func (storage storage) messagesIn(domainId string, month string, conversationId string) ([]core.Message, error) {
	rows, err := storage.stores.Messages.OfConversation(domainId, month, conversationId)
	return rows, storage.wrapped(err)
}

func (storage storage) reactionsIn(domainId string, conversationId string) ([]core.Reaction, error) {
	rows, err := storage.stores.Reactions.OfConversation(domainId, conversationId)
	return rows, storage.wrapped(err)
}

func (storage storage) reaction(domainId string, key string) (core.Reaction, bool, error) {
	reaction, found, err := storage.stores.Reactions.ByKey(domainId, key)
	return reaction, found, storage.wrapped(err)
}

func (storage storage) attachmentUses(domainId string, attachmentId string) ([]core.AttachmentUse, error) {
	rows, err := storage.stores.AttachmentUses.ByAttachment(domainId, attachmentId)
	return rows, storage.wrapped(err)
}

func (storage storage) saveMessage(message core.Message) error {
	return storage.wrapped(storage.stores.Messages.Save(message))
}

func (storage storage) saveParticipant(participant core.Participant) error {
	return storage.wrapped(storage.stores.Participants.Save(participant))
}

func (storage storage) saveReaction(reaction core.Reaction) error {
	return storage.wrapped(storage.stores.Reactions.Save(reaction))
}

func (storage storage) insertReaction(reaction core.Reaction) error {
	_, err := storage.stores.Reactions.Insert(reaction)
	return storage.wrapped(err)
}

func (storage) wrapped(failure error) error {
	if failure == nil {
		return nil
	}
	return &core.Unavailable{Message: "the message store is not available", Cause: failure}
}
