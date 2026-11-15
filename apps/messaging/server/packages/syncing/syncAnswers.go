package syncing

import "mesail.dev/packages/core"

type syncAnswers struct{}

func (syncAnswers) gone(cursor core.SyncCursor) core.ConversationSync {
	return core.ConversationSync{ConversationId: cursor.ConversationId, State: core.SyncStates{}.Gone(), Messages: []core.MessageView{}}
}

func (syncAnswers) unchanged(cursor core.SyncCursor) core.ConversationSync {
	return core.ConversationSync{ConversationId: cursor.ConversationId, State: core.SyncStates{}.Unchanged(), Messages: []core.MessageView{}, Next: cursor.After}
}

func (syncAnswers) reset(cursor core.SyncCursor, newest string) core.ConversationSync {
	return core.ConversationSync{ConversationId: cursor.ConversationId, State: core.SyncStates{}.Reset(), Messages: []core.MessageView{}, Next: newest}
}

func (syncAnswers) changed(cursor core.SyncCursor, views []core.MessageView, page changePage) core.ConversationSync {
	return core.ConversationSync{ConversationId: cursor.ConversationId, State: core.SyncStates{}.Changed(), Messages: views, Next: page.next, More: page.more}
}
