package reading

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"time"

	"mesail.dev/packages/core"
)

type messageHistory struct {
	storage storage
	now     func() time.Time
}

func (history messageHistory) page(conversation core.Conversation, filter messageFilter, limit int) (messagePage, error) {
	months := monthRange{}.back(history.now(), conversation.CreatedMonth)
	collected, err := history.collect(conversation, filter, months, limit)
	return history.cut(history.newestFirst(collected), limit), err
}

func (history messageHistory) collect(conversation core.Conversation, filter messageFilter, months []string, limit int) ([]core.Message, error) {
	kept := []core.Message{}
	failures := []error{}
	for _, month := range months {
		rows, err := history.monthIfRoom(conversation, month, len(kept) > limit)
		kept = append(kept, filter.matching(rows)...)
		failures = append(failures, err)
	}
	return kept, errors.Join(failures...)
}

func (history messageHistory) monthIfRoom(conversation core.Conversation, month string, full bool) ([]core.Message, error) {
	if full {
		return []core.Message{}, nil
	}
	return history.storage.messagesIn(conversation.DomainId, month, core.ConversationIds{}.Wire(conversation))
}

func (history messageHistory) newestFirst(messages []core.Message) []core.Message {
	sorted := append([]core.Message{}, messages...)
	slices.SortStableFunc(sorted, history.compare)
	return sorted
}

func (messageHistory) compare(first core.Message, second core.Message) int {
	return cmp.Compare(second.Sequence, first.Sequence)
}

func (history messageHistory) cut(messages []core.Message, limit int) messagePage {
	shown := append([]core.Message{}, messages[:min(limit, len(messages))]...)
	return messagePage{messages: shown, nextCursor: history.nextCursor(messages, limit)}
}

func (messageHistory) nextCursor(messages []core.Message, limit int) string {
	if len(messages) <= limit {
		return ""
	}
	return strconv.Itoa(messages[limit-1].Sequence)
}
