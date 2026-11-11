package reading

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type Views struct {
	storage storage
}

func NewViews(stores core.Stores) Views {
	return Views{storage: storage{stores: stores}}
}

func (views Views) Of(viewer oidc.Caller, messages []core.Message, conversation core.Conversation) ([]core.MessageView, error) {
	conversationId := core.ConversationIds{}.Wire(conversation)
	reactions, err := views.storage.reactionsIn(conversation.DomainId, conversationId)
	if err != nil {
		return nil, err
	}
	participants, err := views.storage.participantsIn(conversation.DomainId, conversationId)
	if err != nil {
		return nil, err
	}
	built := messageViews{viewer: viewer, reactions: reactionCounts{rows: reactions}, deliveries: deliveryViews{}.forParticipants(participants)}
	return built.all(messages)
}

func (views Views) single(viewer oidc.Caller, message core.Message, conversation core.Conversation) (core.MessageView, error) {
	found, err := views.Of(viewer, []core.Message{message}, conversation)
	if err != nil {
		return core.MessageView{}, err
	}
	return found[0], nil
}
