package syncing

import (
	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/reading"
)

type Sync struct {
	access  core.Access
	window  syncWindow
	history changeHistory
	loader  messageLoader
	views   reading.Views
}

func NewSync(stores core.Stores, ports core.Ports) Sync {
	window := syncWindow{now: ports.Now}
	return Sync{
		access:  core.NewAccess(stores),
		window:  window,
		history: changeHistory{changes: stores.Changes, heads: stores.ChangeHeads, window: window},
		loader:  messageLoader{messages: stores.Messages},
		views:   reading.NewViews(stores),
	}
}

func (sync Sync) Run(caller oidc.Caller, cursors []core.SyncCursor) ([]core.ConversationSync, error) {
	err := syncRules{}.check(cursors)
	if err != nil {
		return nil, err
	}
	return sync.all(caller, cursors)
}

func (sync Sync) all(caller oidc.Caller, cursors []core.SyncCursor) ([]core.ConversationSync, error) {
	answers := []core.ConversationSync{}
	for _, cursor := range cursors {
		answer, err := sync.one(caller, cursor)
		if err != nil {
			return nil, err
		}
		answers = append(answers, answer)
	}
	return answers, nil
}

func (sync Sync) one(caller oidc.Caller, cursor core.SyncCursor) (core.ConversationSync, error) {
	conversation, err := sync.access.Conversation(caller, cursor.ConversationId)
	if (syncRules{}).gone(err) {
		return syncAnswers{}.gone(cursor), nil
	}
	if err != nil {
		return core.ConversationSync{}, err
	}
	return sync.ofConversation(caller, conversation, cursor)
}

func (sync Sync) ofConversation(caller oidc.Caller, conversation core.Conversation, cursor core.SyncCursor) (core.ConversationSync, error) {
	after, usable := sync.window.usable(cursor.After)
	if !usable {
		return sync.reset(conversation, cursor)
	}
	from := sync.window.overlapStart(after)
	rows, err := sync.history.since(conversation, from)
	if err != nil {
		return core.ConversationSync{}, err
	}
	page := changeWindow{}.page(rows, from, after, sync.window.pageSize())
	if !page.fresh {
		return syncAnswers{}.unchanged(cursor), nil
	}
	return sync.changed(caller, conversation, cursor, page)
}

func (sync Sync) reset(conversation core.Conversation, cursor core.SyncCursor) (core.ConversationSync, error) {
	newest, err := sync.history.newest(conversation)
	return syncAnswers{}.reset(cursor, newest), err
}

func (sync Sync) changed(caller oidc.Caller, conversation core.Conversation, cursor core.SyncCursor, page changePage) (core.ConversationSync, error) {
	messages, err := sync.loader.load(conversation, page.messageIds)
	if err != nil {
		return core.ConversationSync{}, err
	}
	views, err := sync.views.Of(caller, messages, conversation)
	return syncAnswers{}.changed(cursor, views, page), err
}
