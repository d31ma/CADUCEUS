package core

import "fmt"

type MemoryMentions struct {
	rows    []Mention
	counter int
}

func NewMemoryMentions() *MemoryMentions {
	return &MemoryMentions{}
}

func (store *MemoryMentions) Insert(row Mention) (string, error) {
	store.counter++
	row.Id = fmt.Sprintf("%012d", store.counter)
	store.rows = append(store.rows, row)
	return row.Id, nil
}

func (store *MemoryMentions) All() []Mention {
	return append([]Mention{}, store.rows...)
}

func (store *MemoryMentions) find(match func(Mention) bool) (Mention, bool, error) {
	for _, row := range store.rows {
		if match(row) {
			return row, true, nil
		}
	}
	return Mention{}, false, nil
}

func (store *MemoryMentions) filter(match func(Mention) bool) ([]Mention, error) {
	kept := []Mention{}
	for _, row := range store.rows {
		kept = append(kept, store.ifMatching(row, match(row))...)
	}
	return kept, nil
}

func (store *MemoryMentions) ifMatching(row Mention, matched bool) []Mention {
	if matched {
		return []Mention{row}
	}
	return nil
}

func (store *MemoryMentions) replace(row Mention) error {
	for index := range store.rows {
		store.rows[index] = store.replaced(store.rows[index], row)
	}
	return nil
}

func (store *MemoryMentions) replaced(old Mention, row Mention) Mention {
	if old.Id == row.Id {
		return row
	}
	return old
}

func (store *MemoryMentions) OfMember(domainId string, memberId string) ([]Mention, error) {
	return store.filter(func(row Mention) bool { return row.DomainId == domainId && row.MemberId == memberId })
}
