package reading

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

type Reactions struct {
	access  core.Access
	storage storage
	members memberParticipants
	stager  core.Stager
	rules   reactionRules
}

func NewReactions(stores core.Stores, ports core.Ports) Reactions {
	storage := storage{stores: stores}
	return Reactions{
		access:  core.NewAccess(stores),
		storage: storage,
		members: memberParticipants{storage: storage},
		stager:  ports.Stager,
		rules:   reactionRules{},
	}
}

func (reactions Reactions) Add(caller oidc.Caller, messageId string, reaction string) error {
	return reactions.change(caller, reactionRequest{messageId: messageId, reaction: reaction, added: true})
}

func (reactions Reactions) Remove(caller oidc.Caller, messageId string, reaction string) error {
	return reactions.change(caller, reactionRequest{messageId: messageId, reaction: reaction, added: false})
}

func (reactions Reactions) change(caller oidc.Caller, request reactionRequest) error {
	err := reactions.rules.check(request.reaction)
	if err != nil {
		return err
	}
	message, _, err := reactions.access.Message(caller, request.messageId)
	if err != nil {
		return err
	}
	row, found, err := reactions.current(reactions.fresh(caller, message, request.reaction))
	if err != nil {
		return err
	}
	return reactions.toggled(row, found, request.added)
}

func (reactions Reactions) fresh(caller oidc.Caller, message core.Message, reaction string) core.Reaction {
	messageId := core.MessageIds{}.Wire(message.Month, message.Id)
	return core.Reaction{
		DomainId:       message.DomainId,
		Key:            reactionKeys{}.of(messageId, caller.MemberId, reaction),
		ConversationId: message.ConversationId,
		MessageId:      messageId,
		MemberId:       caller.MemberId,
		Emoji:          reaction,
	}
}

func (reactions Reactions) current(fresh core.Reaction) (core.Reaction, bool, error) {
	existing, found, err := reactions.storage.reaction(fresh.DomainId, fresh.Key)
	return reactions.chosen(existing, fresh, found), found, err
}

func (Reactions) chosen(existing core.Reaction, fresh core.Reaction, found bool) core.Reaction {
	if found {
		return existing
	}
	return fresh
}

func (reactions Reactions) toggled(row core.Reaction, found bool, wanted bool) error {
	if row.Active == wanted {
		return nil
	}
	return reactions.written(row, found, wanted)
}

func (reactions Reactions) written(row core.Reaction, found bool, wanted bool) error {
	memberIds, err := reactions.members.of(row.DomainId, row.ConversationId)
	if err != nil {
		return err
	}
	row.Active = wanted
	row.OutboxFields, err = reactions.stager.Stage(row.OutboxFields, events.Types{}.ReactionChanged(), row.DomainId, reactions.payload(row, memberIds))
	if err != nil {
		return err
	}
	return reactions.persisted(row, found)
}

func (Reactions) payload(row core.Reaction, memberIds []string) events.ReactionChanged {
	return events.ReactionChanged{
		MessageId:      row.MessageId,
		ConversationId: row.ConversationId,
		MemberId:       row.MemberId,
		Emoji:          row.Emoji,
		Added:          row.Active,
		MemberIds:      memberIds,
	}
}

func (reactions Reactions) persisted(row core.Reaction, found bool) error {
	if found {
		return reactions.storage.saveReaction(row)
	}
	return reactions.storage.insertReaction(row)
}
