package core

import "fmt"

type MemoryThreadKeys struct {
	rows    []ThreadKey
	counter int
}

func NewMemoryThreadKeys() *MemoryThreadKeys {
	return &MemoryThreadKeys{}
}

func (store *MemoryThreadKeys) Insert(row ThreadKey) (string, error) {
	store.counter++
	row.Id = fmt.Sprintf("%012d", store.counter)
	store.rows = append(store.rows, row)
	return row.Id, nil
}

func (store *MemoryThreadKeys) All() []ThreadKey {
	return append([]ThreadKey{}, store.rows...)
}

func (store *MemoryThreadKeys) find(match func(ThreadKey) bool) (ThreadKey, bool, error) {
	for _, row := range store.rows {
		if match(row) {
			return row, true, nil
		}
	}
	return ThreadKey{}, false, nil
}

func (store *MemoryThreadKeys) filter(match func(ThreadKey) bool) ([]ThreadKey, error) {
	kept := []ThreadKey{}
	for _, row := range store.rows {
		kept = append(kept, store.ifMatching(row, match(row))...)
	}
	return kept, nil
}

func (store *MemoryThreadKeys) ifMatching(row ThreadKey, matched bool) []ThreadKey {
	if matched {
		return []ThreadKey{row}
	}
	return nil
}

func (store *MemoryThreadKeys) replace(row ThreadKey) error {
	for index := range store.rows {
		store.rows[index] = store.replaced(store.rows[index], row)
	}
	return nil
}

func (store *MemoryThreadKeys) replaced(old ThreadKey, row ThreadKey) ThreadKey {
	if old.Id == row.Id {
		return row
	}
	return old
}

func (store *MemoryThreadKeys) ByKey(domainId string, key string) (ThreadKey, bool, error) {
	return store.find(func(row ThreadKey) bool { return row.DomainId == domainId && row.Key == key })
}
