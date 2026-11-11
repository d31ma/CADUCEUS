package reading

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
)

type Reader struct {
	access        core.Access
	storage       storage
	pages         pageRules
	conversations conversationViews
	history       messageHistory
	threads       threadRoots
	views         Views
}

func NewReader(stores core.Stores, ports core.Ports) Reader {
	storage := storage{stores: stores}
	access := core.NewAccess(stores)
	return Reader{
		access:        access,
		storage:       storage,
		pages:         pageRules{},
		conversations: conversationViews{storage: storage, titles: conversationTitles{storage: storage}},
		history:       messageHistory{storage: storage, now: ports.Now},
		threads:       threadRoots{messages: conversationMessages{access: access}},
		views:         NewViews(stores),
	}
}

func (reader Reader) Conversations(caller oidc.Caller, page core.PageRequest) (core.ConversationList, error) {
	window, err := reader.pages.conversationWindow(page)
	if err != nil {
		return core.ConversationList{}, err
	}
	participations, err := reader.storage.participationsOf(caller.DomainId, caller.MemberId)
	if err != nil {
		return core.ConversationList{}, err
	}
	mentions, err := reader.storage.mentionsOf(caller.DomainId, caller.MemberId)
	if err != nil {
		return core.ConversationList{}, err
	}
	views, err := reader.conversations.build(caller, participations, mentions)
	if err != nil {
		return core.ConversationList{}, err
	}
	return conversationPage{}.of(views, window), nil
}

func (reader Reader) Messages(caller oidc.Caller, conversationId string, threadRootId string, page core.PageRequest) (core.MessageList, error) {
	window, err := reader.pages.messageWindow(page)
	if err != nil {
		return core.MessageList{}, err
	}
	conversation, err := reader.access.Conversation(caller, conversationId)
	if err != nil {
		return core.MessageList{}, err
	}
	err = reader.threads.check(caller, conversation, threadRootId)
	if err != nil {
		return core.MessageList{}, err
	}
	found, err := reader.history.page(conversation, messageFilter{threadRootId: threadRootId, before: window.cursor}, window.limit)
	if err != nil {
		return core.MessageList{}, err
	}
	views, err := reader.views.Of(caller, found.messages, conversation)
	if err != nil {
		return core.MessageList{}, err
	}
	return core.MessageList{Items: views, NextCursor: found.nextCursor}, nil
}

func (reader Reader) Message(caller oidc.Caller, messageId string) (core.MessageView, error) {
	message, conversation, err := reader.access.Message(caller, messageId)
	if err != nil {
		return core.MessageView{}, err
	}
	return reader.views.single(caller, message, conversation)
}
