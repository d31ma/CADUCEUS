package reading

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
)

type Changes struct {
	access  core.Access
	storage storage
	members memberParticipants
	ports   core.Ports
	rules   changeRules
	views   Views
}

func NewChanges(stores core.Stores, ports core.Ports) Changes {
	storage := storage{stores: stores}
	return Changes{
		access:  core.NewAccess(stores),
		storage: storage,
		members: memberParticipants{storage: storage},
		ports:   ports,
		rules:   changeRules{},
		views:   NewViews(stores),
	}
}

func (changes Changes) Edit(caller oidc.Caller, messageId string, body string) (core.MessageView, error) {
	message, conversation, err := changes.access.Message(caller, messageId)
	if err != nil {
		return core.MessageView{}, err
	}
	err = changes.rules.editable(caller, message, body)
	if err != nil {
		return core.MessageView{}, err
	}
	rewritten, err := changes.rewritten(message, body)
	if err != nil {
		return core.MessageView{}, err
	}
	err = core.MessageBudget{}.Check(rewritten.Body, rewritten.BlocksJson)
	if err != nil {
		return core.MessageView{}, err
	}
	saved, err := changes.published(rewritten, events.Types{}.MessageEdited())
	if err != nil {
		return core.MessageView{}, err
	}
	return changes.views.single(caller, saved, conversation)
}

func (changes Changes) Delete(caller oidc.Caller, messageId string) error {
	message, _, err := changes.access.Message(caller, messageId)
	if err != nil {
		return err
	}
	err = changes.rules.author(caller, message)
	if err != nil {
		return err
	}
	return changes.erasedOnce(message)
}

func (changes Changes) rewritten(message core.Message, body string) (core.Message, error) {
	rich, err := changes.ports.Renderer.Markdown(body)
	message.Body = body
	message.BlocksJson = messageBlocks{}.encoded(rich.Blocks)
	message.Truncated = rich.Truncated
	message.EditedAt = clock{now: changes.ports.Now}.stamp()
	return message, err
}

func (changes Changes) erasedOnce(message core.Message) error {
	if message.DeletedAt != "" {
		return nil
	}
	_, err := changes.published(changes.erased(message), events.Types{}.MessageDeleted())
	return err
}

func (changes Changes) erased(message core.Message) core.Message {
	message.Body = ""
	message.BlocksJson = messageBlocks{}.none()
	message.Truncated = false
	message.AttachmentIds = []string{}
	message.AttachmentsJson = core.AttachmentCodec{}.Encode(nil)
	message.DeletedAt = clock{now: changes.ports.Now}.stamp()
	return message
}

func (changes Changes) published(message core.Message, eventType string) (core.Message, error) {
	memberIds, err := changes.members.of(message.DomainId, message.ConversationId)
	if err != nil {
		return core.Message{}, err
	}
	message.OutboxFields, err = changes.ports.Stager.Stage(message.OutboxFields, eventType, message.DomainId, changes.payload(message, memberIds, eventType))
	if err != nil {
		return core.Message{}, err
	}
	return message, changes.storage.saveMessage(message)
}

func (changes Changes) payload(message core.Message, memberIds []string, eventType string) any {
	if eventType == (events.Types{}).MessageDeleted() {
		return changeEvents{}.deleted(message, memberIds)
	}
	return changeEvents{}.edited(message, memberIds)
}
