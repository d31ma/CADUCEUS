package core

import "fmt"

type MemoryChangeHeads struct {
	rows    []ChangeHead
	counter int
}

func NewMemoryChangeHeads() *MemoryChangeHeads {
	return &MemoryChangeHeads{}
}

func (store *MemoryChangeHeads) Insert(row ChangeHead) (string, error) {
	store.counter++
	row.Id = fmt.Sprintf("%012d", store.counter)
	store.rows = append(store.rows, row)
	return row.Id, nil
}

func (store *MemoryChangeHeads) Save(row ChangeHead) error {
	for index := range store.rows {
		store.rows[index] = store.replaced(store.rows[index], row)
	}
	return nil
}

func (store *MemoryChangeHeads) Find(domainId string, conversationId string) (ChangeHead, bool, error) {
	for _, row := range store.rows {
		if row.DomainId == domainId && row.ConversationId == conversationId {
			return row, true, nil
		}
	}
	return ChangeHead{}, false, nil
}

func (store *MemoryChangeHeads) All() []ChangeHead {
	return append([]ChangeHead{}, store.rows...)
}

func (store *MemoryChangeHeads) replaced(old ChangeHead, row ChangeHead) ChangeHead {
	if old.Id == row.Id {
		return row
	}
	return old
}
