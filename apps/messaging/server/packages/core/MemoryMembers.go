package core

import "fmt"

type MemoryMembers struct {
	rows    []Member
	counter int
}

func NewMemoryMembers() *MemoryMembers {
	return &MemoryMembers{}
}

func (store *MemoryMembers) Insert(row Member) (string, error) {
	store.counter++
	row.Id = fmt.Sprintf("%012d", store.counter)
	store.rows = append(store.rows, row)
	return row.Id, nil
}

func (store *MemoryMembers) All() []Member {
	return append([]Member{}, store.rows...)
}

func (store *MemoryMembers) find(match func(Member) bool) (Member, bool, error) {
	for _, row := range store.rows {
		if match(row) {
			return row, true, nil
		}
	}
	return Member{}, false, nil
}

func (store *MemoryMembers) filter(match func(Member) bool) ([]Member, error) {
	kept := []Member{}
	for _, row := range store.rows {
		kept = append(kept, store.ifMatching(row, match(row))...)
	}
	return kept, nil
}

func (store *MemoryMembers) ifMatching(row Member, matched bool) []Member {
	if matched {
		return []Member{row}
	}
	return nil
}

func (store *MemoryMembers) replace(row Member) error {
	for index := range store.rows {
		store.rows[index] = store.replaced(store.rows[index], row)
	}
	return nil
}

func (store *MemoryMembers) replaced(old Member, row Member) Member {
	if old.Id == row.Id {
		return row
	}
	return old
}

func (store *MemoryMembers) Save(member Member) error {
	existing, found, _ := store.ByMemberId(member.DomainId, member.MemberId)
	return store.upsert(member, existing, found)
}

func (store *MemoryMembers) ByMemberId(domainId string, memberId string) (Member, bool, error) {
	return store.find(func(row Member) bool { return row.DomainId == domainId && row.MemberId == memberId })
}

func (store *MemoryMembers) ByLocalPart(domainId string, localPart string) (Member, bool, error) {
	return store.find(func(row Member) bool { return row.DomainId == domainId && row.LocalPart == localPart })
}

func (store *MemoryMembers) upsert(member Member, existing Member, found bool) error {
	if found {
		member.Id = existing.Id
		return store.replace(member)
	}
	_, err := store.Insert(member)
	return err
}
