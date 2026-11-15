package core

import "fmt"

type MemoryChanges struct {
	rows    []Change
	counter int
}

func NewMemoryChanges() *MemoryChanges {
	return &MemoryChanges{}
}

func (store *MemoryChanges) Append(row Change) (string, error) {
	store.counter++
	row.Id = fmt.Sprintf("%012d", store.counter)
	store.rows = append(store.rows, row)
	return row.Id, nil
}

func (store *MemoryChanges) All() []Change {
	return append([]Change{}, store.rows...)
}

func (store *MemoryChanges) OfEvent(domainId string, month string, eventId string) ([]Change, error) {
	return store.filter(func(row Change) bool {
		return row.DomainId == domainId && row.Month == month && row.EventId == eventId
	}), nil
}

func (store *MemoryChanges) OfConversation(domainId string, month string, conversationId string) ([]Change, error) {
	return store.filter(func(row Change) bool {
		return row.DomainId == domainId && row.Month == month && row.ConversationId == conversationId
	}), nil
}

func (store *MemoryChanges) filter(match func(Change) bool) []Change {
	kept := []Change{}
	for _, row := range store.rows {
		kept = append(kept, store.ifMatching(row, match(row))...)
	}
	return kept
}

func (store *MemoryChanges) ifMatching(row Change, matched bool) []Change {
	if matched {
		return []Change{row}
	}
	return nil
}
