package syncing

import (
	"encoding/json"

	"mesail.dev/packages/events"
)

type changeEntries struct{}

func (entries changeEntries) of(envelope events.Envelope) ([]changeEntry, error) {
	if !entries.logged(envelope.Type) {
		return nil, nil
	}
	changed := changedMessage{}
	err := json.Unmarshal(envelope.Payload, &changed)
	return entries.listed(changed), err
}

func (changeEntries) logged(eventType string) bool {
	types := events.Types{}
	switch eventType {
	case types.MessageCreated(), types.MessageEdited(), types.MessageDeleted(), types.ReactionChanged():
		return true
	}
	return false
}

func (changeEntries) listed(changed changedMessage) []changeEntry {
	listed := []changeEntry{{conversationId: changed.ConversationId, messageId: changed.MessageId}}
	if changed.ThreadRootId == "" {
		return listed
	}
	return append(listed, changeEntry{conversationId: changed.ConversationId, messageId: changed.ThreadRootId})
}
