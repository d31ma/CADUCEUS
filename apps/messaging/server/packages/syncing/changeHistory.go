package syncing

import (
	"errors"
	"time"

	"mesail.dev/packages/core"
)

type changeHistory struct {
	changes core.ChangeStore
	heads   core.ChangeHeadStore
	window  syncWindow
}

func (history changeHistory) since(conversation core.Conversation, from time.Time) ([]core.Change, error) {
	conversationId := core.ConversationIds{}.Wire(conversation)
	found := []core.Change{}
	failures := []error{}
	for _, month := range history.window.months(from) {
		rows, err := history.changes.OfConversation(conversation.DomainId, month, conversationId)
		found, failures = append(found, rows...), append(failures, err)
	}
	return found, history.unavailable(errors.Join(failures...))
}

func (history changeHistory) newest(conversation core.Conversation) (string, error) {
	head, _, err := history.heads.Find(conversation.DomainId, core.ConversationIds{}.Wire(conversation))
	return head.LastChangeId, history.unavailable(err)
}

func (changeHistory) unavailable(failure error) error {
	if failure == nil {
		return nil
	}
	return core.Unavailable{}.Store(failure)
}
