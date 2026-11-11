package sending

import (
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type ConversationPicker struct {
	stores core.Stores
	direct DirectConversations
}

func NewConversationPicker(stores core.Stores, now time.Time) ConversationPicker {
	return ConversationPicker{stores: stores, direct: NewDirectConversations(stores, now)}
}

func (picker ConversationPicker) Pick(caller oidc.Caller, request core.SendRequest, chosen []Recipient) (Placement, error) {
	return picker.route(request.ConversationId)(caller, request, chosen)
}

func (picker ConversationPicker) route(conversationId string) func(oidc.Caller, core.SendRequest, []Recipient) (Placement, error) {
	if conversationId != "" {
		return picker.existing
	}
	return picker.started
}

func (picker ConversationPicker) started(caller oidc.Caller, request core.SendRequest, chosen []Recipient) (Placement, error) {
	return picker.startRoute(chosen[0].Kind)(caller, request, chosen)
}

func (picker ConversationPicker) startRoute(kind string) func(oidc.Caller, core.SendRequest, []Recipient) (Placement, error) {
	if kind == (RecipientCheck{}).channelKind() {
		return picker.channel
	}
	return picker.direct.pick
}

func (picker ConversationPicker) existing(caller oidc.Caller, request core.SendRequest, chosen []Recipient) (Placement, error) {
	conversation, sender, err := core.NewAccess(picker.stores).Postable(caller, request.ConversationId)
	if err != nil {
		return Placement{}, err
	}
	err = picker.extraRefusal(conversation, chosen)
	if err != nil {
		return Placement{}, err
	}
	stored, err := picker.participants(caller.DomainId, conversation)
	if err != nil {
		return Placement{}, err
	}
	added := Newcomers{}.of(stored, chosen)
	joined := picker.joinedKey(picker.joinedExternal(conversation, Newcomers{}.external(added)), len(added))
	return Placement{Conversation: joined, Existing: stored, Newcomers: added, Sender: sender}, nil
}

func (picker ConversationPicker) channel(caller oidc.Caller, request core.SendRequest, chosen []Recipient) (Placement, error) {
	stored, found, err := picker.stores.Conversations.ByChannel(caller.DomainId, chosen[0].ChannelId)
	if err != nil {
		return Placement{}, Failures{}.store(err)
	}
	err = picker.channelRefusal(found)
	if err != nil {
		return Placement{}, err
	}
	conversation, sender, err := core.NewAccess(picker.stores).Postable(caller, core.ConversationIds{}.Wire(stored))
	if err != nil {
		return Placement{}, err
	}
	members, err := picker.participants(caller.DomainId, conversation)
	return Placement{Conversation: conversation, Existing: members, Sender: sender}, err
}

func (picker ConversationPicker) participants(domainId string, conversation core.Conversation) ([]core.Participant, error) {
	stored, err := picker.stores.Participants.OfConversation(domainId, core.ConversationIds{}.Wire(conversation))
	if err != nil {
		return nil, Failures{}.store(err)
	}
	return stored, nil
}

func (ConversationPicker) extraRefusal(conversation core.Conversation, chosen []Recipient) error {
	if len(chosen) == 0 || conversation.Kind == (core.ConversationKinds{}).Direct() {
		return nil
	}
	return Failures{}.recipients("recipients can be added only to a direct conversation")
}

func (ConversationPicker) channelRefusal(found bool) error {
	if found {
		return nil
	}
	return Failures{}.missingConversation()
}

func (ConversationPicker) joinedExternal(conversation core.Conversation, external bool) core.Conversation {
	conversation.HasExternal = conversation.HasExternal || external
	return conversation
}

func (ConversationPicker) joinedKey(conversation core.Conversation, addedCount int) core.Conversation {
	if addedCount > 0 {
		conversation.ParticipantKey = ""
	}
	return conversation
}
