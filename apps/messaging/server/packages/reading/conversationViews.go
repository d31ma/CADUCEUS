package reading

import (
	"errors"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type conversationViews struct {
	storage storage
	titles  conversationTitles
}

func (views conversationViews) build(caller oidc.Caller, participations []core.Participant, mentions []core.Mention) ([]core.ConversationView, error) {
	built := []core.ConversationView{}
	failures := []error{}
	for _, participant := range participations {
		found, err := views.one(caller, participant, mentions)
		built = append(built, found...)
		failures = append(failures, err)
	}
	return built, errors.Join(failures...)
}

func (views conversationViews) one(caller oidc.Caller, participant core.Participant, mentions []core.Mention) ([]core.ConversationView, error) {
	conversation, found, err := views.lookup(caller.DomainId, participant.ConversationId)
	if err != nil {
		return nil, err
	}
	view, err := views.view(caller, participant, conversation, mentions)
	return views.kept(view, found), err
}

func (views conversationViews) lookup(domainId string, conversationId string) (core.Conversation, bool, error) {
	storedPart, err := core.ConversationIds{}.Parse(conversationId)
	if err != nil {
		return core.Conversation{}, false, nil
	}
	return views.storage.conversation(domainId, storedPart)
}

func (views conversationViews) view(caller oidc.Caller, participant core.Participant, conversation core.Conversation, mentions []core.Mention) (core.ConversationView, error) {
	title, err := views.titles.of(caller, conversation)
	return core.ConversationView{
		Conversation:   conversation,
		ConversationId: core.ConversationIds{}.Wire(conversation),
		Title:          title,
		UnreadCount:    max(0, conversation.MessageCount-participant.ReadSequence),
		MentionCount:   views.mentionCount(mentions, participant),
	}, err
}

func (views conversationViews) mentionCount(mentions []core.Mention, participant core.Participant) int {
	count := 0
	for _, mention := range mentions {
		count += views.unreadMention(mention, participant)
	}
	return count
}

func (conversationViews) unreadMention(mention core.Mention, participant core.Participant) int {
	if mention.ConversationId == participant.ConversationId && mention.Sequence > participant.ReadSequence {
		return 1
	}
	return 0
}

func (conversationViews) kept(view core.ConversationView, found bool) []core.ConversationView {
	if found {
		return []core.ConversationView{view}
	}
	return nil
}
