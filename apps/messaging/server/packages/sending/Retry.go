package sending

import "mesail.dev/packages/core"

type Retry struct {
	stores core.Stores
	events MessageEvents
	uses   AttachmentUses
}

func NewRetry(stores core.Stores, stager core.Stager) Retry {
	return Retry{stores: stores, events: NewMessageEvents(stager), uses: NewAttachmentUses(stores.AttachmentUses)}
}

func (retry Retry) complete(message core.Message) (core.Message, error) {
	return retry.route(message.EventsStaged)(message)
}

func (retry Retry) route(eventsStaged bool) func(core.Message) (core.Message, error) {
	if eventsStaged {
		return retry.done
	}
	return retry.restage
}

func (Retry) done(message core.Message) (core.Message, error) {
	return message, nil
}

func (retry Retry) restage(message core.Message) (core.Message, error) {
	storedPart, err := core.ConversationIds{}.Parse(message.ConversationId)
	if err != nil {
		return message, err
	}
	conversation, _, err := retry.stores.Conversations.Find(message.DomainId, storedPart)
	if err != nil {
		return message, Failures{}.store(err)
	}
	participants, err := retry.stores.Participants.OfConversation(message.DomainId, message.ConversationId)
	if err != nil {
		return message, Failures{}.store(err)
	}
	err = retry.uses.write(message)
	if err != nil {
		return message, err
	}
	staged, err := retry.events.stage(message, conversation, Readers{}.of(participants))
	if err != nil {
		return message, err
	}
	err = retry.stores.Messages.Save(staged)
	if err != nil {
		return message, Failures{}.store(err)
	}
	return staged, nil
}
