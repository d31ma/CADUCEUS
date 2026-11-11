package core

import "fmt"

type MemoryReactions struct {
	rows    []Reaction
	counter int
}

func NewMemoryReactions() *MemoryReactions {
	return &MemoryReactions{}
}

func (store *MemoryReactions) Insert(row Reaction) (string, error) {
	store.counter++
	row.Id = fmt.Sprintf("%012d", store.counter)
	store.rows = append(store.rows, row)
	return row.Id, nil
}

func (store *MemoryReactions) All() []Reaction {
	return append([]Reaction{}, store.rows...)
}

func (store *MemoryReactions) find(match func(Reaction) bool) (Reaction, bool, error) {
	for _, row := range store.rows {
		if match(row) {
			return row, true, nil
		}
	}
	return Reaction{}, false, nil
}

func (store *MemoryReactions) filter(match func(Reaction) bool) ([]Reaction, error) {
	kept := []Reaction{}
	for _, row := range store.rows {
		kept = append(kept, store.ifMatching(row, match(row))...)
	}
	return kept, nil
}

func (store *MemoryReactions) ifMatching(row Reaction, matched bool) []Reaction {
	if matched {
		return []Reaction{row}
	}
	return nil
}

func (store *MemoryReactions) replace(row Reaction) error {
	for index := range store.rows {
		store.rows[index] = store.replaced(store.rows[index], row)
	}
	return nil
}

func (store *MemoryReactions) replaced(old Reaction, row Reaction) Reaction {
	if old.Id == row.Id {
		return row
	}
	return old
}

func (store *MemoryReactions) Save(reaction Reaction) error {
	return store.replace(reaction)
}

func (store *MemoryReactions) ByKey(domainId string, key string) (Reaction, bool, error) {
	return store.find(func(row Reaction) bool { return row.DomainId == domainId && row.Key == key })
}

func (store *MemoryReactions) OfConversation(domainId string, conversationId string) ([]Reaction, error) {
	return store.filter(func(row Reaction) bool {
		return row.DomainId == domainId && row.ConversationId == conversationId
	})
}
